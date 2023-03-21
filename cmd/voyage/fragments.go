package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

var fragmentSchema = `
CREATE TABLE IF NOT EXISTS fragments (
    id INTEGER PRIMARY KEY ,
    text TEXT NOT NULL,
    created_at TEXT NOT NULL
);
`

type Image struct {
	ID        int      `db:"id" json:"id"`
	Path      string   `db:"path" json:"path"`
	CreatedAt string   `db:"created_at" json:"created_at"`
	Prompt    string   `db:"prompt" json:"prompt"`
	Tags      []string `db:"tags" json:"tags"`
}

func listImages(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sb := sqlbuilder.NewSelectBuilder()
		sb.Select("*").From("images").OrderBy("created_at DESC")
		sql, args := sb.Build()

		images := []Image{}
		err := db.Select(&images, sql, args...)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error occurred while fetching images")
			return
		}

		for i, image := range images {
			sb := sqlbuilder.NewSelectBuilder()
			sb.Select("t.tag").From("tags t")
			sb.JoinWithOption("", "image_tags it", "it.tag_id = t.id")
			sb.Where(sb.Equal("it.image_id", image.ID))

			sql, args := sb.Build()

			var tags []string
			err = db.Select(&tags, sql, args...)

			if err != nil {
				c.String(http.StatusInternalServerError, "Error occurred while fetching tags")
				return
			}

			images[i].Tags = tags
		}

		c.HTML(http.StatusOK, "images.html", gin.H{
			"images": images,
		})
	}
}

func saveImage(fileHeader *multipart.FileHeader) (string, error) {
	// Create the uploads directory if it doesn't exist
	err := os.MkdirAll("upload/images", os.ModePerm)
	if err != nil {
		return "", err
	}

	// Generate a unique name for the file
	dst := fmt.Sprintf("upload/images/%s-%s", uuid.New().String(), fileHeader.Filename)

	// Save the file to the uploads directory
	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	if err != nil {
		return "", err
	}

	return dst, nil
}

func createImage(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read the uploaded image
		file, err := c.FormFile("image")
		if err != nil {
			c.String(http.StatusBadRequest, "Image field is required")
			return
		}

		// Save the image to the upload/images directory
		path, err := saveImage(file)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error occurred while saving the image")
			return
		}

		// Read the other form fields
		prompt := c.PostForm("prompt")
		tagsStr := c.PostForm("tags")
		tags := strings.Split(tagsStr, ",")

		// Insert the image into the database
		now := time.Now().Format(time.RFC3339)
		sb := sqlbuilder.NewInsertBuilder()
		sb.InsertInto("images")
		sb.Cols("path", "created_at", "prompt")
		sb.Values(path, now, prompt)
		sql, args := sb.Build()

		result, err := db.Exec(sql, args...)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error occurred while creating image")
			return
		}

		imageID, _ := result.LastInsertId()

		// Insert tags into the database and associate them with the image
		for _, tag := range tags {
			tagID := 0
			err := db.Get(&tagID, "SELECT id FROM tags WHERE tag = ?", tag)
			if err == sql.ErrNoRows {
				// Insert the new tag
				res, err := db.Exec("INSERT INTO tags (tag) VALUES (?)", tag)
				if err != nil {
					c.String(http.StatusInternalServerError, "Error occurred while creating tag")
					return
				}
				tagID64, _ := res.LastInsertId()
				tagID = int(tagID64)
			} else if err != nil {
				c.String(http.StatusInternalServerError, "Error occurred while fetching tag")
				return
			}

			// Associate the tag with the image
			_, err = db.Exec("INSERT INTO image_tags (image_id, tag_id) VALUES (?, ?)", imageID, tagID)
			if err != nil {
				c.String(http.StatusInternalServerError, "Error occurred while linking tag and image")
				return
			}
		}

		// Render the HTML snippet for htmx
		c.HTML(http.StatusCreated, "image.html", gin.H{
			"id":        imageID,
			"path":      path,
			"createdAt": now,
			"prompt":    prompt,
			"tags":      tags,
		})
	}
}

type Fragment struct {
	ID        int    `db:"id" json:"id"`
	Text      string `db:"text" json:"text"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

func listFragments(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		fragments := []Fragment{}
		err := db.Select(&fragments, "SELECT * FROM fragments")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, fragments)
	}
}

func getFragment(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var fragment Fragment

		err := db.Get(&fragment, "SELECT * FROM fragments WHERE id = ?", id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fragment not found"})
			return
		}

		c.HTML(http.StatusOK, "fragment.tmpl.html", fragment)
	}
}

func createFragment(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var fragment Fragment
		if err := c.ShouldBindJSON(&fragment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		now := time.Now().Format(time.RFC3339)
		fragment.CreatedAt = now

		sb := sqlbuilder.NewInsertBuilder()
		sb.InsertInto("fragments")
		sb.Cols("text", "created_at")
		sb.Values(fragment.Text, fragment.CreatedAt)
		sql, args := sb.Build()

		result, err := db.Exec(sql, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		id, _ := result.LastInsertId()
		fragment.ID = int(id)
		c.JSON(http.StatusCreated, fragment)
	}
}

func updateFragment(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// check the requested content-type
		if c.ContentType() != "application/json" {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "Content-Type must be application/json"})
			return
		}

		id := c.Param("id")
		var fragment Fragment
		err := db.Get(&fragment, "SELECT * FROM fragments WHERE id = ?", id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fragment not found"})
			return
		}

		if err := c.ShouldBindJSON(&fragment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err = db.Exec("UPDATE fragments SET text = ? WHERE id = ?", fragment.Text, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.HTML(http.StatusOK, "fragment.tmpl.html", fragment)
	}
}

func deleteFragment(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		result, err := db.Exec("DELETE FROM fragments WHERE id = ?", id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Fragment not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Fragment with ID %s deleted", id)})
	}
}
