package service

import "mime/multipart"

func ParseUpload(f *multipart.FileHeader)([]BulkRow,error){return parseUpload(f)}
