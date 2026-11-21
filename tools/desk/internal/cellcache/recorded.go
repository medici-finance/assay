package cellcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// RecordedReport returns the measurement the cache last recorded, for a read-only report
// (`cellctl gc`'s dry run). It takes no lock and walks nothing. It reads report.json only
// through readMetaBounded, the reader every in-package reader of that file uses: no link is
// followed, a FIFO cannot stall it, and the read is capped. An error wrapping fs.ErrNotExist
// means the cache has not been measured yet.
func RecordedReport(p Policy) (Report, error) {
	if err := p.Validate(); err != nil {
		return Report{}, err
	}
	if err := validatePath(p.Root); err != nil {
		return Report{}, err
	}
	r, err := os.OpenRoot(p.Root)
	if err != nil {
		return Report{}, err
	}
	defer r.Close()
	st, err := r.Stat(".")
	if err != nil {
		return Report{}, err
	}
	dev, _, err := identity(st)
	if err != nil {
		return Report{}, err
	}
	b, err := readMetaBounded(r, dev, "report.json", 64<<10)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Report{}, err
		}
		return Report{}, fmt.Errorf("recorded cache report not readable: %w", err)
	}
	var rep Report
	if err := json.Unmarshal(b, &rep); err != nil || rep.Schema != schema {
		return Report{}, errors.New("recorded cache report is not parseable or not in this schema")
	}
	return rep, nil
}
