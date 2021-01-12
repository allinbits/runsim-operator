package simulation

const (
	DefaultMinioEndpoint  = "s3.amazonaws.com"
	DefaultLogsBucketName = "simulation-logs"
)

func defaultOptions() *Options {
	return &Options{
		LogBackupEnabled: false,
		MinioEndpoint:    DefaultMinioEndpoint,
		LogsBucketName:   DefaultLogsBucketName,
	}
}

type Options struct {
	LogBackupEnabled  bool
	MinioEndpoint     string
	LogsBucketName    string
	S3AccessKeyId     string
	S3SecretAccessKey string
	ImagePullSecret   string
}

type Option func(*Options)

func EnableLogBackups(v bool) Option {
	return func(opts *Options) {
		opts.LogBackupEnabled = v
	}
}

func MinioEndpoint(s string) Option {
	return func(opts *Options) {
		opts.MinioEndpoint = s
	}
}

func LogsBucketName(s string) Option {
	return func(opts *Options) {
		opts.LogsBucketName = s
	}
}

func S3AccessKeyId(s string) Option {
	return func(opts *Options) {
		opts.S3AccessKeyId = s
	}
}

func S3SecretAccessKey(s string) Option {
	return func(opts *Options) {
		opts.S3SecretAccessKey = s
	}
}

func WithImagePullSecret(s string) Option {
	return func(opts *Options) {
		opts.ImagePullSecret = s
	}
}
