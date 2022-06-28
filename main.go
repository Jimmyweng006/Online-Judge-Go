package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// custom map type
type problemsMap map[string]string

// Day 5 delete problem by ID api
func remove(slice []Problem, s int) []Problem {
	return append(slice[:s], slice[s+1:]...)
}

func main() {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, Jimmy_kiet.")
	})

	// fake data
	var testProblems []Problem
	p1t1 := TestCase{
		Input:          "3 4",
		ExpectedOutput: "7",
		Comment:        "",
		Score:          50,
		TimeOutSeconds: 10.0,
	}

	p1t2 := TestCase{
		Input:          "2147483646 1",
		ExpectedOutput: "2147483647",
		Comment:        "",
		Score:          50,
		TimeOutSeconds: 10.0,
	}

	p1 := Problem{
		Id:          "101",
		Title:       "A + B Problem",
		Description: "輸入兩數，將兩數加總。",
		TestCases:   []TestCase{p1t1, p1t2},
	}

	p2t1 := TestCase{
		Input:          "3 4 5",
		ExpectedOutput: "12",
		Comment:        "",
		Score:          50,
		TimeOutSeconds: 10.0,
	}

	p2t2 := TestCase{
		Input:          "2147483646 1 -1",
		ExpectedOutput: "2147483646",
		Comment:        "",
		Score:          50,
		TimeOutSeconds: 10.0,
	}

	p2 := Problem{
		Id:          "102",
		Title:       "A + B + C Problem",
		Description: "輸入三數，將三數加總。",
		TestCases:   []TestCase{p2t1, p2t2},
	}

	testProblems = append(testProblems, p1)
	testProblems = append(testProblems, p2)

	// group: problems
	getProblemsHandler := func(c *gin.Context) {
		// slice of map
		var problems []problemsMap
		for _, p := range testProblems {
			temp := problemsMap{
				"id":    p.Id,
				"title": p.Title,
			}
			problems = append(problems, temp)
		}

		c.JSON(http.StatusOK, gin.H{
			"data": problems,
		})
	}

	createProblemsHandler := func(c *gin.Context) {
		var newProblem Problem
		err := c.Bind(&newProblem)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("create problem err: %s", err.Error()))
		}
		testProblems = append(testProblems, newProblem)

		c.JSON(http.StatusOK, gin.H{
			"Ok": true,
		})
	}

	getProblemByIDHandler := func(c *gin.Context) {
		problemId := c.Param("id")
		// slice of map
		var problems []problemsMap
		for _, p := range testProblems {
			if p.Id == problemId {
				temp := problemsMap{
					"id":    p.Id,
					"title": p.Title,
				}
				problems = append(problems, temp)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"problem": problems,
		})
	}

	updateProblemByIDHandler := func(c *gin.Context) {
		problemId := c.Param("id")
		var updatedProblem Problem
		err := c.Bind(&updatedProblem)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("update problem err: %s", err.Error()))
		}

		for _, p := range testProblems {
			if p.Id == problemId {
				p = updatedProblem
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"Ok": true,
		})
	}

	deleteProblemByIDHandler := func(c *gin.Context) {
		problemId := c.Param("id")

		for i, p := range testProblems {
			if p.Id == problemId {
				testProblems = remove(testProblems, i)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"Ok": true,
		})
	}

	problems := r.Group("/problems")
	{
		problems.GET("/", getProblemsHandler)
		problems.POST("/", createProblemsHandler)

		problems.GET("/:id", getProblemByIDHandler)
		problems.PUT("/:id", updateProblemByIDHandler)
		problems.DELETE("/:id", deleteProblemByIDHandler)
	}

	r.Run() // listen and serve on 0.0.0.0:8080
}
