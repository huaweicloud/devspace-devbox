// Pointer helpers for the optional fields of the oapi-codegen generated
// request/response types. Kept next to the types they exist for.

package api

// PInt returns a pointer to v as an int.
func PInt(v int32) *int {
	i := int(v)
	return &i
}

// PInt32 returns a pointer to v.
func PInt32(v int32) *int32 { return &v }

// PString returns a pointer to s.
func PString(s string) *string { return &s }

// PStringMap returns a pointer to m, or nil when m is empty so the field is
// omitted from JSON bodies.
func PStringMap(m map[string]string) *map[string]string {
	if len(m) == 0 {
		return nil
	}
	return &m
}

// OptString returns a pointer to s, or nil when s is empty.
func OptString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
