package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// custom map type
type problemsMap map[string]string

const userKey = "user"

func initDatabase() (db *gorm.DB, err error) {
	dsn := "host=localhost user=postgres password=123456789 " +
		"dbname=onlinejudge-go port=5432 sslmode=disable"
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	return db, err
}

func encryptBySha256(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:])
}

func verifyPassword(password string, dbPassword string) bool {
	return encryptBySha256(password) == dbPassword
}

func authorize(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(userKey)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.Next()
}

func main() {
	db, err := initDatabase()
	if err != nil {
		fmt.Println(err)
		return
	}

	// create tables
	db.Transaction(func(tx *gorm.DB) error {
		tx.AutoMigrate(&ProblemTable{}, &TestCaseTable{}, &UserTable{})

		return nil
	})

	r := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, Jimmy_kiet.")
	})

	// group: problems
	getProblemsHandler := func(c *gin.Context) {
		var problems []problemsMap

		db.Transaction(func(tx *gorm.DB) error {
			// do some database operations in the transaction (use 'tx' from this point, not 'db')
			rows, err := tx.Model(&ProblemTable{}).Rows()
			defer rows.Close()
			if err != nil {
				fmt.Println(err)
				return err
			}

			for rows.Next() {
				var problem ProblemTable
				// ScanRows is a method of `gorm.DB`, it can be used to scan a row into a struct
				tx.ScanRows(rows, &problem)

				// do something
				temp := problemsMap{
					"id":    strconv.Itoa(problem.Id),
					"title": problem.Title,
				}
				problems = append(problems, temp)
			}

			// return nil will commit the whole transaction
			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"data": problems,
		})
	}

	createProblemsHandler := func(c *gin.Context) {
		var newProblemDTO ProblemPostDTO
		var newProblem ProblemTable
		var newProblemId int

		err := c.Bind(&newProblemDTO)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("create problem err: %s", err.Error()))
			return
		}
		newProblem = ProblemTable{
			Title:       newProblemDTO.Title,
			Description: newProblemDTO.Description,
		}

		db.Transaction(func(tx *gorm.DB) error {
			tx.Create(&newProblem)
			newProblemId = newProblem.Id

			for _, TestCase := range newProblemDTO.TestCases {
				tempTestCase := TestCaseTable{
					Input:          TestCase.Input,
					ExpectedOutput: TestCase.ExpectedOutput,
					Comment:        TestCase.Comment,
					Score:          TestCase.Score,
					TimeOutSeconds: TestCase.TimeOutSeconds,
					ProblemId:      newProblemId,
				}
				tx.Create(&tempTestCase)
			}

			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"problem_id": newProblemId,
		})
	}

	getProblemByIDHandler := func(c *gin.Context) {
		problemId, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get problem Id err: %s", err.Error()))
			return
		}

		var responseData Problem
		var requesetProblem ProblemTable
		db.Transaction(func(tx *gorm.DB) error {
			db.First(&requesetProblem, problemId)
			if requesetProblem.Id == 0 {
				return nil
			}

			rows, err := tx.Model(&TestCaseTable{ProblemId: problemId}).Rows()
			defer rows.Close()
			if err != nil {
				fmt.Println(err)
				return err
			}

			var requestTestcases []TestCase
			for rows.Next() {
				var testcase TestCaseTable
				tx.ScanRows(rows, &testcase)

				temp := TestCase{
					Id:             strconv.Itoa(testcase.Id),
					Input:          testcase.Input,
					ExpectedOutput: testcase.ExpectedOutput,
					Comment:        testcase.Comment,
					Score:          testcase.Score,
					TimeOutSeconds: testcase.TimeOutSeconds,
				}

				requestTestcases = append(requestTestcases, temp)
			}

			responseData = Problem{
				Id:          strconv.Itoa(requesetProblem.Id),
				Title:       requesetProblem.Title,
				Description: requesetProblem.Description,
				TestCases:   requestTestcases,
			}

			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"data": responseData,
		})
	}

	/* three cases
	1. use map to record new testcases
	2. iterate on old testcases
	2-1. if cur testcase not in new testcases set, <delete cur testcase>
	3. iterate on new testcases
	3-1. if cur testcase id is empty(string -> ""), <create cur testcase>
	3-2. else, <update cur testcase>
	*/
	updateProblemByIDHandler := func(c *gin.Context) {
		problemId, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get problem Id err: %s", err.Error()))
			return
		}

		var updatedProblem ProblemPutDTO
		err = c.Bind(&updatedProblem)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("update problem err: %s", err.Error()))
			return
		}

		// record new testcases
		newTestcasesMap := map[string]TestCasePutDTO{}
		for _, t := range updatedProblem.TestCases {
			if t.Id != "" {
				newTestcasesMap[t.Id] = t
			}
		}

		db.Transaction(func(tx *gorm.DB) error {
			// update Problem details
			result := tx.Model(&ProblemTable{Id: problemId}).Updates(
				ProblemTable{Title: updatedProblem.Title, Description: updatedProblem.Description})
			if result.RowsAffected == 0 {
				fmt.Println("no corresponding row found")
				return err
			}

			rows, err := tx.Model(&TestCaseTable{ProblemId: problemId}).Rows()
			defer rows.Close()
			if err != nil {
				fmt.Println(err)
				return err
			}

			// delete cur testcase
			var deletedTestcases []TestCasePutDTO
			for rows.Next() {
				var testcase TestCaseTable
				tx.ScanRows(rows, &testcase)

				_, ok := newTestcasesMap[strconv.Itoa(testcase.Id)]
				temp := TestCasePutDTO{
					Id: strconv.Itoa(testcase.Id),
				}

				if !ok {
					deletedTestcases = append(deletedTestcases, temp)
				}
			}
			for _, deletedTestcase := range deletedTestcases {
				deletedId, err := strconv.Atoi(deletedTestcase.Id)
				if err != nil {
					fmt.Println("delete error")
					return err
				}

				tx.Delete(&TestCaseTable{Id: deletedId})
			}

			// create & update cur testcase
			for _, t := range updatedProblem.TestCases {
				if t.Id == "" {
					testcase := TestCaseTable{
						Input:          t.Input,
						ExpectedOutput: t.ExpectedOutput,
						Comment:        t.Comment,
						Score:          t.Score,
						TimeOutSeconds: t.TimeOutSeconds,
						ProblemId:      problemId,
					}

					tx.Create(&testcase)
				} else {
					updatedId, err := strconv.Atoi(t.Id)
					if err != nil {
						fmt.Println("update error")
						return err
					}

					tx.Model(&TestCaseTable{Id: updatedId}).Updates(
						TestCaseTable{
							Input:          t.Input,
							ExpectedOutput: t.ExpectedOutput,
							Comment:        t.Comment,
							Score:          t.Score,
							TimeOutSeconds: t.TimeOutSeconds,
						})
				}
			}

			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"Ok": true,
		})
	}

	deleteProblemByIDHandler := func(c *gin.Context) {
		problemId, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get problem Id err: %s", err.Error()))
			return
		}

		db.Transaction(func(tx *gorm.DB) error {
			tx.Where("problem_id = ?", problemId).Delete(&TestCaseTable{})
			tx.Delete(&ProblemTable{}, problemId)

			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"Ok": true,
		})
	}

	problems := r.Group("/problems")
	{
		problems.GET("/", getProblemsHandler)
		problems.GET("/:id", getProblemByIDHandler)
	}
	problems.Use(authorize)
	{
		problems.POST("/", createProblemsHandler)
		problems.PUT("/:id", updateProblemByIDHandler)
		problems.DELETE("/:id", deleteProblemByIDHandler)
	}

	createUserHandler := func(c *gin.Context) {
		var newUserDTO UserPostDTO
		var newUser UserTable
		var newUserId int

		err := c.Bind(&newUserDTO)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("create user err: %s", err.Error()))
			return
		}
		newUser = UserTable{
			Username:  newUserDTO.Username,
			Password:  encryptBySha256(newUserDTO.Password),
			Name:      newUserDTO.Name,
			Email:     newUserDTO.Email,
			Authority: 1,
		}

		db.Transaction(func(tx *gorm.DB) error {
			tx.Create(&newUser)
			newUserId = newUser.Id

			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"user_id": newUserId,
		})
	}

	loginHandler := func(c *gin.Context) {
		var userLoginDTO UserLoginDTO
		var userId int
		var authority int
		session := sessions.Default(c)

		err := c.Bind(&userLoginDTO)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("login err: %s", err.Error()))
			return
		}

		var requesetUser UserTable
		dbError := false
		db.Transaction(func(tx *gorm.DB) error {
			tx.Where(&UserTable{Username: userLoginDTO.Username}).First(&requesetUser)
			if requesetUser.Id == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user not exist"})
				dbError = true
				return nil
			}

			if !verifyPassword(userLoginDTO.Password, requesetUser.Password) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user's password doesn't match"})
				dbError = true
				return nil
			}

			userId = requesetUser.Id
			authority = requesetUser.Authority
			return nil
		})
		if dbError == true {
			return
		}

		session.Set(userKey, userId)
		if err := session.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"user_id":        userId,
			"user_authority": authority,
		})
	}

	logoutHandler := func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get(userKey)
		if user == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session token"})
			return
		}

		session.Delete(userKey)
		if err := session.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
	}

	users := r.Group("/users")
	{
		users.POST("/", createUserHandler)
		users.POST("/login", loginHandler)
		users.POST("/logout", logoutHandler)
	}

	r.Run() // listen and serve on 0.0.0.0:8080
}
