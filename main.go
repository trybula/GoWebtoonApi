package main

/// Koreader asks for job --GET--> server responds with status and job nr --RESPONSE--> Koreader saves the job nr and asks for the status --GET--> Server responds with percent, when 100% file becomes available at /get_your_file/:id-->korader downloads a file using GET /getyourfile/:jobid

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"webtoon_dl_web/lib"

	"github.com/gin-gonic/gin"
)

type dl_job struct {
	ID       int      `json:"id"`
	Url      string   `json:"url"`
	Start    int      `json:"start_ep"`
	End      int      `json:"end_ep"`
	Progress int      `json:"progress"`
	Files    []string `json:"files"`
}

var jobsMu sync.Mutex

var jobs = []dl_job{}

func dump_jobs(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, jobs)
}

func post_job(c *gin.Context) {
	var new_job dl_job

	if err := c.ShouldBindJSON(&new_job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	if !strings.HasPrefix(new_job.Url, "https://www.webtoons.com") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid url"})
		return
	}
	//now we are sure we have correct json
	new_job.ID = rand.Intn(1000)
	if new_job.End == 0 {
		new_job.End = math.MaxInt
	}
	new_job.Progress = 0 //na wypadek jakby ktos przeslal
	jobs = append(jobs, new_job)

	// return job id as JSON
	c.JSON(http.StatusAccepted, gin.H{"job_id": new_job.ID})
}

func start_job(c *gin.Context) {
	id, err := strconv.Atoi(strings.ReplaceAll(c.Param("id"), "/", ""))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, err)
		return
	}

	// find job and copy needed fields under lock
	jobsMu.Lock()
	idx := -1
	var url string
	var start, end int
	for i, j := range jobs {
		if j.ID == id {
			idx = i
			url = j.Url
			start = j.Start
			end = j.End
			break
		}
	}
	jobsMu.Unlock()

	if idx == -1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job does not exist"})
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
				for i := range jobs {
					if jobs[i].ID == jobID {
						jobs[i].Progress = -1
						break
					}
				}
				// Optional: You might want to update the job status to "Failed" here
				// so it doesn't stay stuck forever.
				// Note: You need to handle locking carefully here if you access 'jobs'.
			}
		}()

		// 2. Your original logic
		var p int
		// If this function panics, execution jumps immediately to the defer above
		files := lib.DownloadGiven(url, start, end, &p)

		jobsMu.Lock()
		defer jobsMu.Unlock() // This handles unlocking even if code below crashes

		for i := range jobs {
			if jobs[i].ID == jobID {
				jobs[i].Files = files
				jobs[i].Progress = p
				break
			}
		}
	}(id)
}

func delete_job(c *gin.Context) {
	id, err := strconv.Atoi(strings.ReplaceAll(c.Param("id"), "/", ""))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, err)
	} else {
		for i, j := range jobs {
			if j.ID == id {

				for _, n := range j.Files {
					os.Remove("./comics/" + n)
				}

				jobs = slices.Delete(jobs, i, i+1)
				c.IndentedJSON(http.StatusOK, fmt.Sprintf("job %d removed", id))
				return
			}
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "job does not exist"})
	}
}

func get_job_info(c *gin.Context) {
	id, err := strconv.Atoi(strings.ReplaceAll(c.Param("id"), "/", ""))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, err)
	} else {
		for i, j := range jobs {
			if j.ID == id {
				c.IndentedJSON(http.StatusOK, jobs[i])
				return
			}
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "job does not exist"})
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)

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
