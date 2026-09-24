package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleOpenWebUIUsers handles /api/v1/users/*
func (h *APIHandler) handleOpenWebUIUsers(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users")
	path = strings.TrimPrefix(path, "/")

	// 1. Delegate /user or root to handleOpenWebUIAuth
	if path == "" || path == "user" || path == "user/" {
		h.handleOpenWebUIAuth(w, r)
		return
	}

	// 2. Profile Image: /users/{id}/profile/image or /users/user/profile/image
	if strings.HasSuffix(path, "/profile/image") {
		targetUserID := strings.TrimSuffix(path, "/profile/image")
		targetUserID = strings.Trim(targetUserID, "/")
		if targetUserID == "" || targetUserID == "user" {
			targetUserID = u.ID
		}

		if r.Method == http.MethodGet {
			p, err := h.db.GetOpenWebUIUserProfile(subdomain, targetUserID)
			if err == nil && p != nil && p.ProfileImageURL != "" {
				// Handle base64 data URL
				if strings.HasPrefix(p.ProfileImageURL, "data:image/") {
					parts := strings.SplitN(p.ProfileImageURL, ",", 2)
					if len(parts) == 2 {
						header := parts[0]
						rawB64 := parts[1]
						mediaType := "image/png"
						if semiIdx := strings.Index(header, ";"); semiIdx != -1 {
							mediaType = strings.TrimPrefix(header[:semiIdx], "data:")
						}
						decoded, decErr := base64.StdEncoding.DecodeString(rawB64)
						if decErr == nil {
							w.Header().Set("Content-Type", mediaType)
							w.Header().Set("Content-Disposition", "inline")
							w.Header().Set("X-Content-Type-Options", "nosniff")
							w.Header().Set("Cache-Control", "public, max-age=86400")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(decoded)
							return
						}
					}
				}

				// Handle remote HTTP URL redirect
				if strings.HasPrefix(p.ProfileImageURL, "http://") || strings.HasPrefix(p.ProfileImageURL, "https://") {
					http.Redirect(w, r, p.ProfileImageURL, http.StatusFound)
					return
				}
			}

			// Fallback to user.png
			owuDir := h.locateOpenWebUIDir()
			if owuDir != "" {
				userPng := filepath.Join(owuDir, "user.png")
				if stat, err := os.Stat(userPng); err == nil && !stat.IsDir() {
					http.ServeFile(w, r, userPng)
					return
				}
			}
			http.Redirect(w, r, "/user.png", http.StatusFound)
			return
		}

		if r.Method == http.MethodPost {
			// Handle multipart image upload
			if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
				err := r.ParseMultipartForm(10 << 20) // 10MB max avatar
				if err == nil {
					file, _, err := r.FormFile("file")
					if err != nil {
						file, _, err = r.FormFile("image")
					}
					if err == nil {
						defer file.Close()
						bytes, _ := io.ReadAll(file)
						mime := http.DetectContentType(bytes)
						b64 := base64.StdEncoding.EncodeToString(bytes)
						dataURL := fmt.Sprintf("data:%s;base64,%s", mime, b64)

						_ = h.db.UpsertOpenWebUIUserProfile(subdomain, targetUserID, "", dataURL, "", "", "")

						writeJSON(w, http.StatusOK, map[string]interface{}{
							"status":            true,
							"profile_image_url": dataURL,
						})
						return
					}
				}
			}

			// Handle JSON payload
			body, _ := io.ReadAll(r.Body)
			var form struct {
				ProfileImageURL string `json:"profile_image_url"`
			}
			_ = json.Unmarshal(body, &form)
			if form.ProfileImageURL != "" {
				_ = h.db.UpsertOpenWebUIUserProfile(subdomain, targetUserID, "", form.ProfileImageURL, "", "", "")
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status":            true,
				"profile_image_url": form.ProfileImageURL,
			})
			return
		}
	}

	// 3. Update User: /users/{id}/update or /users/user/update or /users/user/info/update
	if strings.HasSuffix(path, "/update") {
		targetUserID := strings.TrimSuffix(path, "/update")
		targetUserID = strings.TrimSuffix(targetUserID, "/info")
		targetUserID = strings.Trim(targetUserID, "/")
		if targetUserID == "" || targetUserID == "user" {
			targetUserID = u.ID
		}

		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var form struct {
				Name            string `json:"name"`
				ProfileImageURL string `json:"profile_image_url"`
				Bio             string `json:"bio"`
				Gender          string `json:"gender"`
				DateOfBirth     string `json:"date_of_birth"`
			}
			_ = json.Unmarshal(body, &form)

			name := form.Name
			if name == "" {
				name = u.Name
			}
			profileImage := form.ProfileImageURL
			if profileImage == "" {
				profileImage = u.ProfileImageURL
			}

			_ = h.db.UpsertOpenWebUIUserProfile(subdomain, targetUserID, name, profileImage, form.Bio, form.Gender, form.DateOfBirth)

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":                targetUserID,
				"name":              name,
				"email":             u.Email,
				"role":              u.Role,
				"profile_image_url": profileImage,
				"bio":               form.Bio,
				"gender":            form.Gender,
				"date_of_birth":     form.DateOfBirth,
			})
			return
		}
	}

	// 4. GET /users/all or /users
	if path == "all" || path == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"users": []interface{}{u},
			"total": 1,
		})
		return
	}

	// 5. GET /users/{id}/active
	if strings.HasSuffix(path, "/active") {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"active": true,
		})
		return
	}

	// 6. GET /users/{id}
	targetUserID := strings.Trim(path, "/")
	if targetUserID == "user" || targetUserID == u.ID {
		writeJSON(w, http.StatusOK, u)
		return
	}

	name := "User " + targetUserID
	if len(targetUserID) > 8 {
		name = "User " + targetUserID[:4]
	}
	profileImage := fmt.Sprintf("/api/v1/users/%s/profile/image", targetUserID)
	var bio, gender, dob *string
	if p, err := h.db.GetOpenWebUIUserProfile(subdomain, targetUserID); err == nil && p != nil {
		if p.Name != "" {
			name = p.Name
		}
		if p.ProfileImageURL != "" {
			profileImage = p.ProfileImageURL
		}
		if p.Bio != "" {
			bio = &p.Bio
		}
		if p.Gender != "" {
			gender = &p.Gender
		}
		if p.DateOfBirth != "" {
			dob = &p.DateOfBirth
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":                targetUserID,
		"name":              name,
		"role":              "user",
		"email":             fmt.Sprintf("%s@%s.router.zcdns.id", targetUserID, subdomain),
		"profile_image_url": profileImage,
		"bio":               bio,
		"gender":            gender,
		"date_of_birth":     dob,
		"is_active":         true,
	})
}
