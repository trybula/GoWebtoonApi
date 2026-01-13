package main

/// Koreader asks for job --GET--> server responds with status and job nr --RESPONSE--> Koreader saves the job nr and asks for the status --GET--> Server responds with percent, when 100% file becomes available at /get_your_file/:id-->korader downloads a file using GET /getyourfile/:jobid

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"webtoon_dl_web/lib"

	"database/sql"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
)

type dl_job struct {
	ID       int      `json:"id"`
	Url      string   `json:"url"`
	Start    int      `json:"start_ep"`
	End      int      `json:"end_ep"`
	Progress int      `json:"progress"`
	Files    []string `json:"files"`
}

var db *sql.DB

func dump_jobs(c *gin.Context) {
	c.Status(http.StatusOK)
}

func post_job(c *gin.Context) {
	var new_job dl_job

	if err := c.ShouldBindJSON(&new_job); err != nil {
		rawBody, _ := c.GetRawData()
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		slog.Warn("Error while binding json", "body", string(rawBody), "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if !strings.HasPrefix(new_job.Url, "https://www.webtoons.com") {
		slog.Warn("Got request with wrong url", "url", new_job.Url)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid url"})
		return
	}
	//now we are sure we have correct json
	//new_job.ID = rand.Intn(1000)
	/*if new_job.End == 0 {
		new_job.End = math.MaxInt
	}
	new_job.Progress = 0 //na wypadek jakby ktos przeslal

	jobs = append(jobs, new_job)
	*/
	id_number, err := DbInsert(new_job)
	if err != nil {
		//fmt.Println(err)
		c.JSON(http.StatusTeapot, gin.H{"error": "insertion error"})
		return
	}
	//fmt.Printf("Added to db with id: %d\n", id_number)
	// return job id as JSON
	c.JSON(http.StatusAccepted, gin.H{"job_id": id_number})
}

// przerob na db
func start_job(c *gin.Context) {
	id, err := strconv.Atoi(strings.ReplaceAll(c.Param("id"), "/", ""))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, err)
		return
	}
	// return immediately
	c.Status(http.StatusOK)

	// run download in background (do not use c or take pointers into jobs slice)
	go func(jobID int) {
		// 1. THIS MUST BE FIRST
		// This function will execute when the surrounding function finishes
		// (either naturally or via panic)
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in Job %d: %v\n", jobID, r)
				sql := "UPDATE jobs SET Progress=? WHERE Id=?"
				//_ = db.QueryRow(sql, -1, id)
				if _, err := db.Exec(sql, -1, jobID); err != nil {
					slog.Error("failed to update job on panic", "id", jobID, "error", err.Error())
				}
			}
		}()

		lib.DownloadGiven(db, jobID)

	}(id)
}

func delete_job(c *gin.Context) {
	id, err := strconv.Atoi(strings.ReplaceAll(c.Param("id"), "/", ""))
	if err != nil {
		slog.Warn("error while searching for id in url", "param", c.Param("id"))
		c.Status(http.StatusBadRequest)
	} else {
		files, err := split_db_to_files(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "db error"})
			return
		}
		for _, file := range files {
			os.Remove("./comics/" + file)
			slog.Debug("Removing file", "file", "./comics/"+file)
		}

		sql := `DELETE FROM jobs WHERE Id = ?;`
		_, err = db.Exec(sql, id)
		if err != nil {
			slog.Error("DB error while deleting job", "error", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "db error while deleting"})
			return
		} else {
			slog.Debug("job removed", "id", id)
			c.IndentedJSON(http.StatusOK, fmt.Sprintf("job %d removed", id))
			return
		}
	}
}

func get_job_info(c *gin.Context) {
	id, err := strconv.Atoi(strings.ReplaceAll(c.Param("id"), "/", ""))
	if err != nil {
		//c.IndentedJSON(http.StatusBadRequest, "error with request")
		slog.Warn("error while searching for id in url", "param", c.Param("id"))
		c.Status(http.StatusBadRequest)
	} else {
		job, err := DbGet(id)
		if err != nil {
			//fmt.Println(err)
			slog.Warn("Get job info failed", "error", err.Error(), "id", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "job does not exist or other db err"})
			return
		}
		c.IndentedJSON(http.StatusOK, job)
		return
		/*
			for i, j := range jobs {
				if j.ID == id {
					c.IndentedJSON(http.StatusOK, jobs[i])
					return
				}
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "job does not exist"})
		*/
	}
}

func CreateTable() {

	sql := `CREATE TABLE IF NOT EXISTS jobs (
        Id   INTEGER PRIMARY KEY AUTOINCREMENT,
        Url  TEXT NOT NULL,
        Start INTEGER DEFAULT 0,
        "End" INTEGER DEFAULT 10,
		Progress INTEGER DEFAULT 0,
        Files TEXT DEFAULT ""
    );`

	_, err := db.Exec(sql)
	if err != nil {
		slog.Error("create table failed:", "error", err.Error())
		return
	}

	slog.Debug("Created new table successfully")
}
func DbInsert(j dl_job) (int, error) {
	sql := `INSERT INTO jobs (Url, Start,End) VALUES(?,?,?);`
	res, err := db.Exec(sql, j.Url, j.Start, j.End)
	if err != nil {
		slog.Warn("Error while inserting into db", "url", j.Url, "Start", j.Start, "End", j.End, "error", err.Error())
		return -1, err
	} else {
		i64val, err := res.LastInsertId()
		slog.Debug("Inserted into db", "id", int(i64val), "url", j.Url, "Start", j.Start, "End", j.End)
		return int(i64val), err
	}
}
func DbGet(id int) (*dl_job, error) {
	tmp := &dl_job{}
	sql := `SELECT Id,Url,Start,"End",Progress FROM jobs WHERE Id = ?`
	row := db.QueryRow(sql, id)
	err := row.Scan(&tmp.ID, &tmp.Url, &tmp.Start, &tmp.End, &tmp.Progress)
	if err != nil {
		slog.Warn("Error while requesting db", "Id", id, "error", err.Error())
		return nil, err
	} else {
		tmp.Files, err = split_db_to_files(id)
		if err != nil {
			return nil, err
		}
		return tmp, nil
	}
}
func split_db_to_files(id int) ([]string, error) {
	sql := `SELECT Files FROM jobs WHERE Id = ?`
	row := db.QueryRow(sql, id)
	var txt_files string
	err := row.Scan(&txt_files)
	if err != nil {
		slog.Warn("Error while requesting db", "Id", id, "error", err.Error())
		return nil, err
	}
	var array []string = strings.Split(txt_files, ";")
	if len(array) > 0 && array[len(array)-1] == "" {
		array = array[:len(array)-1]
	}
	return array, nil
}
func main() {
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}
	defer logFile.Close()
	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	slog.SetDefault(logger)
	slog.Info("Logger initalised")

	dbloc, err := sql.Open("sqlite", "./jobs.db")
	if err != nil {
		slog.Error("Failed to open db file", "error", err.Error())
		panic("failed to open db file: " + err.Error())
	}
	db = dbloc
	CreateTable()
	defer db.Close() //will close table at the end

	gin.SetMode(gin.DebugMode)

	loadKeyHash()
	//fmt.Print("Hello World\n")
	router := gin.Default()

	authorized := router.Group("/")
	authorized.Use(AuthMiddleware())
	{
		authorized.POST("/post_job", post_job)
		authorized.GET("/dump_jobs", dump_jobs)
		authorized.DELETE("/delete_job/:id", delete_job)
		authorized.GET("/job_info/:id", get_job_info)
		authorized.GET("/start_job/:id", start_job)
		authorized.Static("/files", "./comics")
	}
	router.Run(":80")
}
