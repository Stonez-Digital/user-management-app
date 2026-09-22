package service
import "mime/multipart"
func parseUploadBridge(f *multipart.FileHeader)([]BulkRow,error){return parseUpload(f)}
func serviceFilename(name string)string{return name}
