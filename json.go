package binding

import (
	"encoding/json"
	"io"
)

// Json is middleware to deserialize a JSON payload from the request
// into the struct that is passed in. The resulting struct is then
// validated, but no error handling is actually performed here.
// An interface pointer can be added as a second argument in order
// to map the struct to a specific interface.
func (vd *Binder) Json(jsonStruct interface{}) Errors {
	var errors Errors
	ensurePointer(jsonStruct)
	if vd.req.Body != nil {
		defer vd.req.Body.Close()
		err := json.NewDecoder(vd.req.Body).Decode(jsonStruct)
		if err != nil && err != io.EOF {
			errors.Add([]string{}, ERR_DESERIALIZATION, err.Error())
		}
	}
	errors = append(errors, vd.Validate(jsonStruct)...)
	return errors
}
