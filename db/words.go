package db

type Words struct {
	RusToSrb map[string]string
	SrbToRus map[string]string
	Indexes  map[int]string
}
