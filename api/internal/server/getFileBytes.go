package server

import "mime/multipart"

func getFileBytes(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, file.Size)
	_, err = f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
