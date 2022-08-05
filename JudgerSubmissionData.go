package main

type JudgerSubmissionData struct {
	Id        int                  `json:"submissionId"`
	Language  string               `json:"language"`
	Code      string               `json:"code"`
	TestCases []JudgerTestCaseData `json:"testCases"`
}

type JudgerTestCaseData struct {
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expectedOutput"`
	Score          int     `json:"score"`
	TimeOutSeconds float64 `json:"timeOutSeconds"`
}
