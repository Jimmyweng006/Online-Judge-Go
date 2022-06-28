package main

type TestCase struct {
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expectedOutput"`
	Comment        string  `json:"comment"`
	Score          int     `json:"score"`
	TimeOutSeconds float64 `json:"timeOutSeconds"`
}
