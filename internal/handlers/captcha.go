package handlers

import (
	"strconv"
	"time"

	"math/rand"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	maxDifficulty = 5
	failsPerLevel = 2 // Number of fails before increasing difficulty
)

func generateProblem(difficulty int) (problem string, answer int) {
	rand.Seed(time.Now().UnixNano())

	switch difficulty {
	case 1:
		a := rand.Intn(8) + 2
		b := rand.Intn(8) + 2
		if rand.Intn(2) == 0 {
			return strconv.Itoa(a) + " × " + strconv.Itoa(b) + " = ?", a * b
		} else {
			c := a * b
			return strconv.Itoa(c) + " - " + strconv.Itoa(a) + " = ?", c - a
		}

	case 2:
		a := rand.Intn(6) + 2
		b := rand.Intn(6) + 2
		c := rand.Intn(3) + 2
		if rand.Intn(2) == 0 {
			return "(" + strconv.Itoa(a) + " + " + strconv.Itoa(b) + ") × " + strconv.Itoa(c) + " = ?", (a + b) * c
		} else {
			return strconv.Itoa(a) + " × " + strconv.Itoa(b) + " + " + strconv.Itoa(c) + " = ?", a*b + c
		}

	case 3:
		a := rand.Intn(5) + 2
		b := rand.Intn(5) + 2
		c := rand.Intn(5) + 2
		ops := []string{"+", "×"}
		op1 := ops[rand.Intn(2)]
		op2 := ops[rand.Intn(2)]

		problem := strconv.Itoa(a) + " " + op1 + " " + strconv.Itoa(b) + " " + op2 + " " + strconv.Itoa(c) + " = ?"

		if op1 == "×" && op2 == "+" {
			return problem, a*b + c
		} else if op1 == "+" && op2 == "×" {
			return problem, a + (b * c)
		} else if op1 == "×" && op2 == "×" {
			return problem, a * b * c
		} else {
			return problem, a + b + c
		}

	case 4:
		a := rand.Intn(6) + 2
		b := rand.Intn(6) + 2
		c := rand.Intn(4) + 2
		return "(" + strconv.Itoa(a) + " + " + strconv.Itoa(b) + ") × " + strconv.Itoa(c) + " = ?", (a + b) * c

	case 5:
		a := rand.Intn(6) + 2
		b := rand.Intn(6) + 2
		c := rand.Intn(3) + 2
		d := rand.Intn(6) + 2
		return "(" + strconv.Itoa(a) + " × " + strconv.Itoa(b) + " + " + strconv.Itoa(c) + ") - " + strconv.Itoa(d) + " = ?",
			(a*b + c) - d

	default:
		a := rand.Intn(6) + 2
		b := rand.Intn(6) + 2
		return strconv.Itoa(a) + " × " + strconv.Itoa(b) + " = ?", a * b
	}
}

func getDifficulty(session sessions.Session) int {
	failedAttempts := session.Get("captcha_fails")
	if failedAttempts == nil {
		return 1
	}

	difficulty := (failedAttempts.(int) / failsPerLevel) + 1
	if difficulty > maxDifficulty {
		difficulty = maxDifficulty
	}
	return difficulty
}

func GenerateCaptcha(c *gin.Context) {
	session := sessions.Default(c)

	difficulty := getDifficulty(session)

	problem, answer := generateProblem(difficulty)

	session.Set("captcha_answer", answer)
	session.Set("captcha_difficulty", difficulty)
	session.Save()

	c.Set("captcha_problem", problem)
	c.Set("captcha_difficulty", difficulty)
}

func IncrementFailedAttempts(session sessions.Session) {
	fails := 0
	if f := session.Get("captcha_fails"); f != nil {
		fails = f.(int)
	}
	session.Set("captcha_fails", fails+1)
	session.Save()
}

func ResetFailedAttempts(session sessions.Session) {
	session.Delete("captcha_fails")
	session.Save()
}
