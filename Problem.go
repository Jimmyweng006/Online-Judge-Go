package main

type Problem struct {
	Id          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	TestCases   []TestCase `json:"testCases"`
}
