package main

type SubmissionTable struct {
	Id           int     `gorm:"auto_increment;primary_key;" json:"submissionId"`
	Language     string  `gorm:"size:255" json:"language"`
	Code         string  `json:"code"`
	ExecutedTime float64 `json:"executedTime"`
	Result       string  `gorm:"size:255" json:"result"`

	ProblemId int `json:"problemId"`
	UserId    int `json:"userId"`
}

type SubmissionPostDTO struct {
	Language  string `gorm:"size:255" json:"language"`
	Code      string `json:"code"`
	ProblemId int    `json:"problemId"`
}

type Submission struct {
	Id           int     `json:"submissionId"`
	Language     string  `json:"language"`
	Code         string  `json:"code"`
	ExecutedTime float64 `json:"executedTime"`
	Result       string  `json:"result"`
	ProblemId    int     `json:"problemId"`
	UserId       int     `json:"userId"`
}
