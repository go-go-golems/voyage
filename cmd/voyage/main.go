package main

import (
	"embed"
	_ "embed"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strconv"
)

//go:embed static/*
var staticFS embed.FS

func findFragment(id int) *Fragment {
	for _, fragment := range fragments {
		if fragment.ID == id {
			return fragment
		}
	}
	return nil
}

func initDB() *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", "fragments.db")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(fragmentSchema)
	if err != nil {
		panic(err)
	}

	return db
}

func main() {
	router := gin.Default()

	// server index.tmpl.html from staticFS on /
	//router.GET("/", func(c *gin.Context) {
	//	c.FileFromFS("/static/index.tmpl.html", http.FS(staticFS))
	//})

	router.LoadHTMLGlob("cmd/voyage/static/*.htm*")

	router.POST("/create-fragment", createFragment)
	// /fragment/$id/edit -> edit fragment
	router.GET("/fragment/:id/edit", func(c *gin.Context) {
		idString := c.Param("id")
		// convert idString to int
		id, err := strconv.Atoi(idString)
		if err != nil {
			c.or(http.StatusBadRequest, err)
			return
		}

		lock.Lock()
		defer lock.Unlock()
		fragment := findFragment(id)
		if fragment == nil {
			c.AbortWithError(http.StatusNotFound, fmt.Errorf("fragment %s not found", id))
			return
		}
		c.HTML(http.StatusOK, "fragment-edit.tmpl.html", fragment)
	})
	router.POST("/fragment/:id/edit", func(c *gin.Context) {
		idString := c.Param("id")
		// convert idString to int
		id, err := strconv.Atoi(idString)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		text := c.PostForm("text")

		lock.Lock()
		defer lock.Unlock()
		fragment := findFragment(id)
		if fragment == nil {
			c.AbortWithError(http.StatusNotFound, fmt.Errorf("fragment %s not found", id))
			return
		}
		fragment.Text = text
		c.HTML(http.StatusOK, "fragment.tmpl.html", fragment)
	})

	// server the file cmd/voyage/static/index.tmpl.html as /
	router.GET("/", func(c *gin.Context) {
		lock.Lock()
		defer lock.Unlock()
		c.HTML(http.StatusOK, "index.tmpl.html", map[string]interface{}{
			"Fragments": fragments,
		})
	})

	// Handle the image upload
	router.POST("/api/image/upload", func(c *gin.Context) {
		file, err := c.FormFile("image")
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		// Save the file to /tmp
		filename := "/tmp/" + file.Filename
		if err := c.SaveUploadedFile(file, filename); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Return the URL to the uploaded image
		c.JSON(http.StatusOK, gin.H{
			"url": "http://localhost:8080/uploads/" + file.Filename,
		})
	})

	// Serve uploaded images
	router.Static("/uploads", "/tmp")

	// Serve the HTML file
	router.StaticFS("/static", http.FS(staticFS))

	router.Run(":8080")
}
