package main

type TestCase struct {
	Id             string  `json:"id"`
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expectedOutput"`
	Comment        string  `json:"comment"`
	Score          int     `json:"score"`
	TimeOutSeconds float64 `json:"timeOutSeconds"`
}

type TestCaseTable struct {
	Id             int     `gorm:"auto_increment;primary_key;" json:"testcaseId"`
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expectedOutput"`
	Comment        string  `json:"comment"`
	Score          int     `json:"score"`
	TimeOutSeconds float64 `json:"timeOutSeconds"`

	ProblemId int `gorm:"foreignKey:ProblemId" json:"problemId"`
}

type TestCasePostDTO struct {
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expectedOutput"`
	Comment        string  `json:"comment"`
	Score          int     `json:"score"`
	TimeOutSeconds float64 `json:"timeOutSeconds"`
}

type TestCasePutDTO struct {
	Id             string  `json:"id"`
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expectedOutput"`
	Comment        string  `json:"comment"`
	Score          int     `json:"score"`
	TimeOutSeconds float64 `json:"timeOutSeconds"`
}
