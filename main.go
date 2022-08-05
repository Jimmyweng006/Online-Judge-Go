package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// custom map type
type problemsMap map[string]string

const userKey = "user"
const SUBMISSION_NO_RESULT = "-"
const SUPPORTED_LANGUAGE = "kotlin"

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

func authorizeNormalUser(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(userKey)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.Next()
}

func authorizeSuperUser(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(userKey)

	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	authority, err := strconv.Atoi(user.(UserIdAuthorityPrincipal).Authority)
	if err != nil {
		c.String(http.StatusUnauthorized, fmt.Sprintf("get authority err: %s", err.Error()))
		return
	}
	if authority < 2 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.Next()
}

func getConnection(rdb *redis.Client) error {
	ctx := context.Background()
	pong, err := rdb.Ping(ctx).Result()

	if err != nil {
		fmt.Println("ping error, try reconnect", err.Error())
		rdb = redis.NewClient(&redis.Options{})
		pong, err = rdb.Ping(ctx).Result()
		return err
	}

	fmt.Println("ping result:", pong)
	return nil
}

func main() {
	// init for session encode
	gob.Register(UserIdAuthorityPrincipal{})

	db, err := initDatabase()
	if err != nil {
		fmt.Println(err)
		return
	}

	rdb := redis.NewClient(&redis.Options{})
	defer rdb.Close()

	// create tables
	db.Transaction(func(tx *gorm.DB) error {
		tx.AutoMigrate(&ProblemTable{}, &TestCaseTable{}, &UserTable{}, &SubmissionTable{})

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

	createProblemHandler := func(c *gin.Context) {
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
			tx.First(&requesetProblem, problemId)
			if requesetProblem.Id == 0 {
				return nil
			}

			rows, err := tx.Model(&TestCaseTable{}).Where("problem_id = ?", problemId).Rows()
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
	problems.Use(authorizeSuperUser)
	{
		problems.POST("/", createProblemHandler)
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

		curUser := UserIdAuthorityPrincipal{
			UserId:    strconv.Itoa(userId),
			Authority: strconv.Itoa(authority),
		}
		session.Set(userKey, curUser)
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

	createSubmissionHandler := func(c *gin.Context) {
		var newSubmissionDTO SubmissionPostDTO
		var newSubmission SubmissionTable
		var newSubmissionId int
		var testCaseData []JudgerTestCaseData = nil

		session := sessions.Default(c)
		user := session.Get(userKey)

		userId, err := strconv.Atoi(user.(UserIdAuthorityPrincipal).UserId)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get user Id err: %s", err.Error()))
			return
		}

		err = c.Bind(&newSubmissionDTO)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("create submission err: %s", err.Error()))
			return
		}
		newSubmission = SubmissionTable{
			Language:     newSubmissionDTO.Language,
			Code:         newSubmissionDTO.Code,
			ExecutedTime: -1.0,
			Result:       "-",

			ProblemId: newSubmissionDTO.ProblemId,
			UserId:    userId,
		}

		db.Transaction(func(tx *gorm.DB) error {
			tx.Create(&newSubmission)
			newSubmissionId = newSubmission.Id

			rows, err := tx.Model(&TestCaseTable{}).Where("problem_id = ?", newSubmissionDTO.ProblemId).Rows()
			defer rows.Close()
			if err != nil {
				fmt.Println(err)
				return err
			}

			for rows.Next() {
				var testcase TestCaseTable
				tx.ScanRows(rows, &testcase)

				temp := JudgerTestCaseData{
					Input:          testcase.Input,
					ExpectedOutput: testcase.ExpectedOutput,
					Score:          testcase.Score,
					TimeOutSeconds: testcase.TimeOutSeconds,
				}

				testCaseData = append(testCaseData, temp)
			}

			return nil
		})

		if newSubmissionId != 0 && testCaseData != nil {
			if err = getConnection(rdb); err == nil {
				judgerSubmissionData := JudgerSubmissionData{
					Id:        newSubmissionId,
					Language:  newSubmission.Language,
					Code:      newSubmission.Code,
					TestCases: testCaseData,
				}

				// todo: check correctness
				bytes, err := json.Marshal(judgerSubmissionData)
				if err != nil {
					panic(err)
				}

				ctx := context.Background()
				_, err = rdb.RPush(ctx, newSubmission.Language, bytes).Result()
				if err != nil {
					panic(err)
				}
			} else {
				fmt.Println(err)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"submission_id": newSubmissionId,
		})
	}

	getSubmissionByIDHandler := func(c *gin.Context) {
		submissionId, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get submission Id err: %s", err.Error()))
			return
		}

		session := sessions.Default(c)
		user := session.Get(userKey)

		userId, err := strconv.Atoi(user.(UserIdAuthorityPrincipal).UserId)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get user Id err: %s", err.Error()))
			return
		}

		var responseData Submission
		var requesetSubmission SubmissionTable
		matchError := false
		db.Transaction(func(tx *gorm.DB) error {
			tx.First(&requesetSubmission, submissionId)
			if requesetSubmission.Id == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "submissionId not match"})
				matchError = true
				return nil
			}

			if requesetSubmission.UserId != userId {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "user not match"})
				matchError = true
				return nil
			}

			responseData = Submission{
				Id:           requesetSubmission.Id,
				Language:     requesetSubmission.Language,
				Code:         requesetSubmission.Code,
				ExecutedTime: requesetSubmission.ExecutedTime,
				Result:       requesetSubmission.Result,
				ProblemId:    requesetSubmission.ProblemId,
				UserId:       requesetSubmission.UserId,
			}

			return nil
		})

		if matchError {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": responseData,
		})
	}

	restartSubmissionsHandler := func(c *gin.Context) {
		var unjudgedSubmissionDataList []JudgerSubmissionData = nil
		var problem_ids []int
		judgerTestCasesMap := make(map[int][]JudgerTestCaseData)
		submissionsMap := make(map[int][]SubmissionTable)
		isOK := true

		db.Transaction(func(tx *gorm.DB) error {
			// 1. find all unjudged submissoins and its problemId
			rows, err := tx.Model(&SubmissionTable{}).Where("result = ?", SUBMISSION_NO_RESULT).Rows()
			defer rows.Close()
			if err != nil {
				return err
			}

			for rows.Next() {
				var submission SubmissionTable
				tx.ScanRows(rows, &submission)

				problem_ids = append(problem_ids, submission.ProblemId)
				submissionsMap[submission.ProblemId] = append(submissionsMap[submission.ProblemId], submission)
			}
			// 2. find it's related testCases
			rows, err = tx.Model(&TestCaseTable{}).Where("problem_id IN ?", problem_ids).Rows()
			defer rows.Close()
			if err != nil {
				return err
			}

			for rows.Next() {
				var testCase TestCaseTable
				tx.ScanRows(rows, &testCase)

				judgerTestCase := JudgerTestCaseData{
					Input:          testCase.Input,
					ExpectedOutput: testCase.ExpectedOutput,
					Score:          testCase.Score,
					TimeOutSeconds: testCase.TimeOutSeconds,
				}

				judgerTestCasesMap[testCase.ProblemId] = append(judgerTestCasesMap[testCase.ProblemId], judgerTestCase)
			}

			return nil
		})

		// 3. combine to JudgerSubmissionData and push to Redis
		for _, submissions := range submissionsMap {
			for _, submission := range submissions {
				judgerSubmissionData := JudgerSubmissionData{
					Id:        submission.Id,
					Language:  submission.Language,
					Code:      submission.Code,
					TestCases: judgerTestCasesMap[submission.ProblemId],
				}

				unjudgedSubmissionDataList = append(unjudgedSubmissionDataList, judgerSubmissionData)
			}
		}

		if unjudgedSubmissionDataList != nil {
			for _, unjudgedSubmissionData := range unjudgedSubmissionDataList {
				if err = getConnection(rdb); err != nil {
					isOK = false
					c.JSON(http.StatusInternalServerError, gin.H{
						"redis": "disconnection",
					})
					return
				}

				bytes, err := json.Marshal(unjudgedSubmissionData)
				if err != nil {
					panic(err)
				}

				ctx := context.Background()
				_, err = rdb.RPush(ctx, unjudgedSubmissionData.Language, bytes).Result()
				if err != nil {
					panic(err)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"data": isOK,
		})
	}

	restartSubmissionByIDHandler := func(c *gin.Context) {
		submissionId, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get submission Id err: %s", err.Error()))
			return
		}

		session := sessions.Default(c)
		user := session.Get(userKey)

		userId, err := strconv.Atoi(user.(UserIdAuthorityPrincipal).UserId)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("get user Id err: %s", err.Error()))
			return
		}

		var requesetSubmission SubmissionTable
		var judgerTestCases []JudgerTestCaseData
		matchError := false
		isOK := true

		db.Transaction(func(tx *gorm.DB) error {
			// 1. check submission id exist or not
			tx.First(&requesetSubmission, submissionId)
			if requesetSubmission.Id == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "submissionId not match"})
				matchError = true
				return nil
			}

			// 2. check submission.user is cur user or not
			if requesetSubmission.UserId != userId {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "user not match"})
				matchError = true
				return nil
			}

			// 3. find submission related testCases
			rows, err := tx.Model(&TestCaseTable{}).Where("problem_id = ?", requesetSubmission.ProblemId).Rows()
			defer rows.Close()
			if err != nil {
				return err
			}

			for rows.Next() {
				var testCase TestCaseTable
				tx.ScanRows(rows, &testCase)

				judgerTestCase := JudgerTestCaseData{
					Input:          testCase.Input,
					ExpectedOutput: testCase.ExpectedOutput,
					Score:          testCase.Score,
					TimeOutSeconds: testCase.TimeOutSeconds,
				}

				judgerTestCases = append(judgerTestCases, judgerTestCase)
			}

			return nil
		})

		if matchError {
			return
		}

		// 4. combine submission and testCases to JudgerSubmissionData, push JudgerSubmissionData to Redis
		if err = getConnection(rdb); err != nil {
			isOK = false
			c.JSON(http.StatusInternalServerError, gin.H{
				"redis": "disconnection",
			})
			return
		}

		// todo: check correctness
		unjudgedSubmissionData := JudgerSubmissionData{
			Id:        requesetSubmission.Id,
			Language:  requesetSubmission.Language,
			Code:      requesetSubmission.Code,
			TestCases: judgerTestCases,
		}
		bytes, err := json.Marshal(unjudgedSubmissionData)
		if err != nil {
			panic(err)
		}

		ctx := context.Background()
		_, err = rdb.RPush(ctx, unjudgedSubmissionData.Language, bytes).Result()
		if err != nil {
			panic(err)
		}

		c.JSON(http.StatusOK, gin.H{
			"data": isOK,
		})
	}

	submissions := r.Group("/submissions")
	submissions.Use(authorizeNormalUser)
	{
		submissions.POST("/", createSubmissionHandler)
		submissions.GET("/:id", getSubmissionByIDHandler)
		submissions.POST("/:id/restart", restartSubmissionByIDHandler)
	}
	submissions.Use(authorizeSuperUser)
	{
		submissions.POST("/restart", restartSubmissionsHandler)
	}

	r.Run() // listen and serve on 0.0.0.0:8080
}
