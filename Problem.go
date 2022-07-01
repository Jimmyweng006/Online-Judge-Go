package main

type Problem struct {
	Id          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	TestCases   []TestCase `json:"testCases"`
}

type ProblemTable struct {
	Id          int    `gorm:"auto_increment;primary_key;" json:"problemId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// Title       string `gorm:"type:varchar(20)" json:"title"`
	// Description string `gorm:"type:varchar(20)" json:"description"`
}

type ProblemPostDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	TestCases   []TestCasePostDTO `json:"testCases"`
}

type ProblemPutDTO struct {
	Id          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	TestCases   []TestCasePutDTO `json:"testCases"`
}
