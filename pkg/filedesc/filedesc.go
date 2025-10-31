package filedesc

type FileDesc struct {
	FdtID       uint32
	Path        string
	Name        string
	Size        int64
	ContentType string
	Md5         string
}