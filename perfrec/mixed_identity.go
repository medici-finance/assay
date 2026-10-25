package perfrec

// ExcludeMixedSchemaIdentities is the mixed-history reader's pre-filter fence.
// A selected producer never dual-writes: the same install/invocation under two
// schema contracts is an ambiguous identity, not two invocations or permission
// to reinterpret retained history. Exclude every copy conservatively. Historical
// single-version selectors remain unchanged; no identifier leaves this API.
func ExcludeMixedSchemaIdentities(records []Record) ([]Record, int) {
	masks := map[string]uint8{}
	key := func(r Record) (string, uint8) {
		if !IDValid(r.Install) {
			return "", 0
		}
		if r.Schema == SchemaV3 && r.V3 != nil && IDValid(r.V3.Attribution.InvocationID) {
			return r.Install + ":" + r.V3.Attribution.InvocationID, 1
		}
		if r.Schema == SchemaV4 && r.V4 != nil && IDValid(r.V4.InvocationID) {
			return r.Install + ":" + r.V4.InvocationID, 2
		}
		return "", 0
	}
	for _, r := range records {
		k, mask := key(r)
		if mask != 0 {
			masks[k] |= mask
		}
	}
	conflicts := 0
	for _, mask := range masks {
		if mask == 3 {
			conflicts++
		}
	}
	if conflicts == 0 {
		return records, 0
	}
	out := make([]Record, 0, len(records))
	for _, r := range records {
		k, _ := key(r)
		if masks[k] != 3 {
			out = append(out, r)
		}
	}
	return out, conflicts
}
