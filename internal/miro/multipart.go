package miro

import "bytes"

// MultipartBody is a pre-built multipart/form-data body. Pass a non-nil
// *MultipartBody to Do/Post/Patch as the body argument; the client will
// send Body as-is with the supplied ContentType.
//
// Callers construct the body with mime/multipart (multipart.NewWriter)
// and pass writer.FormDataContentType() as ContentType. This keeps the
// transport layer ignorant of upload-specific form shapes (Miro's
// "data"+"resource" convention lives in internal/tools/uploads/).
type MultipartBody struct {
	Body        *bytes.Buffer
	ContentType string
}
