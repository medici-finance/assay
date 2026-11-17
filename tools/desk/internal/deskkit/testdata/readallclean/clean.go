package clean

// Negative control for the discarded-io.ReadAll-error guard: every read here checks or
// returns its error, so the guard must report nothing. This file is never compiled.

import (
	"fmt"
	"io"
	"net/http"
)

func checked(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func ifInit(resp *http.Response) error {
	if _, err := io.ReadAll(resp.Body); err != nil {
		return err
	}
	return nil
}

func returned(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

func wrapped(resp *http.Response) error {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	_ = raw
	return nil
}

func reassignedThenChecked(resp *http.Response) error {
	var err error
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func readBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func wrapperChecked(resp *http.Response) ([]byte, error) {
	raw, err := readBody(resp.Body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
