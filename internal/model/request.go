package model

type Request struct {
	FileName string

	About About `yaml:"about"`

	Time      float64    `yaml:"request_time"`
	URI       string     `yaml:"uri"`
	Method    Method     `yaml:"method"`
	Body      Body       `yaml:"body"`
	Responses []Response `yaml:"responses"`
}
