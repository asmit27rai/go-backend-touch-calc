package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/c4gt/tornado-nginx-go-backend/internal/auth"
	"github.com/c4gt/tornado-nginx-go-backend/internal/email"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	handler *Handler
	service *auth.Service
}

func NewAuthHandler(h *Handler, service *auth.Service) *AuthHandler {
	return &AuthHandler{
		handler: h,
		service: service,
	}
}

// AuthRequest represents the request structure for authentication
type AuthRequest struct {
	Action   string `json:"action" form:"action"`
	Email    string `json:"email" form:"email"`
	Password string `json:"pwd" form:"pwd"`
}

// HandleAuth handles the /iauth endpoint
func (h *AuthHandler) HandleAuth(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	switch req.Action {
	case "login":
		h.handleLogin(c, req.Email, req.Password)
	case "register":
		h.handleRegister(c, req.Email, req.Password)
	case "logout":
		h.HandleLogout(c)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
	}
}

// HandleLogin handles login requests
func (h *AuthHandler) HandleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	h.handleLogin(c, req.Email, req.Password)
}

// HandleRegister handles registration requests
func (h *AuthHandler) HandleRegister(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	h.handleRegister(c, req.Email, req.Password)
}

// HandleLogout handles logout requests
func (h *AuthHandler) HandleLogout(c *gin.Context) {
    fmt.Printf("DEBUG: Logging out user\n")
    h.clearCurrentUser(c)
    
    // Check if it's a JSON request
    if c.GetHeader("Content-Type") == "application/json" {
        c.JSON(http.StatusOK, gin.H{
            "result": "ok",
        })
    } else {
        c.Redirect(http.StatusFound, "/browser")
    }
}

func (h *AuthHandler) handleLogin(c *gin.Context, email, password string) {
    if !auth.ValidateEmail(email) {
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusBadRequest, gin.H{
                "data":   "usererror",
                "result": "fail",
            })
        } else {
            c.HTML(http.StatusBadRequest, "login.html", gin.H{
                "user": nil,
                "error": "Please enter a valid email address",
            })
        }
        return
    }

    authenticated, err := h.service.AuthenticateUser(email, password)
    if err != nil {
        exists, _ := h.service.UserExists(email)
        errorMsg := "Authentication failed"
        if !exists {
            errorMsg = "User does not exist"
        }
        
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "data":   "authfail",
                "result": "fail",
            })
        } else {
            c.HTML(http.StatusUnauthorized, "login.html", gin.H{
                "user": nil,
                "error": errorMsg,
            })
        }
        return
    }

    if authenticated {
        h.setCurrentUser(c, email)
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusOK, gin.H{
                "data":   "success",
                "result": "ok",
            })
        } else {
            // Redirect to landing page instead of /browser
            c.Redirect(http.StatusFound, "/browser")
        }
    } else {
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "data":   "authfail",
                "result": "fail",
            })
        } else {
            c.HTML(http.StatusUnauthorized, "login.html", gin.H{
                "user": nil,
                "error": "Invalid email or password",
            })
        }
    }
}

func (h *AuthHandler) handleRegister(c *gin.Context, email, password string) {
    fmt.Printf("DEBUG: Starting registration for email: %s\n", email)
    
    if !auth.ValidateEmail(email) {
        fmt.Printf("DEBUG: Email validation failed for: %s\n", email)
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusBadRequest, gin.H{
                "data": "usererror",
                "result": "fail",
                "message": "Invalid email format",
            })
        } else {
            c.HTML(http.StatusBadRequest, "register.html", gin.H{
                "user": nil,
                "error": "Please enter a valid email address",
            })
        }
        return
    }

    fmt.Printf("DEBUG: Checking if user exists: %s\n", email)
    exists, err := h.service.UserExists(email)
    if err != nil {
        fmt.Printf("DEBUG: Error checking if user exists: %v\n", err)
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusInternalServerError, gin.H{
                "data": "error",
                "result": "fail",
                "message": "Database error: " + err.Error(),
            })
        } else {
            c.HTML(http.StatusInternalServerError, "register.html", gin.H{
                "user": nil,
                "error": "Database error occurred: " + err.Error(),
            })
        }
        return
    }

    if exists {
        fmt.Printf("DEBUG: User already exists: %s\n", email)
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusConflict, gin.H{
                "data": "userexists",
                "result": "fail",
                "message": "User already exists",
            })
        } else {
            c.HTML(http.StatusConflict, "register.html", gin.H{
                "user": nil,
                "error": "User already exists",
            })
        }
        return
    }

    fmt.Printf("DEBUG: Creating user: %s\n", email)
    err = h.service.CreateUser(email, password)
    if err != nil {
        fmt.Printf("DEBUG: Error creating user: %v\n", err)
        if c.GetHeader("Content-Type") == "application/json" {
            c.JSON(http.StatusInternalServerError, gin.H{
                "data": "error",
                "result": "fail",
                "message": "Failed to create user: " + err.Error(),
            })
        } else {
            c.HTML(http.StatusInternalServerError, "register.html", gin.H{
                "user": nil,
                "error": "Failed to create user: " + err.Error(),
            })
        }
        return
    }

    fmt.Printf("DEBUG: Creating user directories\n")
    // Create user home directory and required directories
    userHomePath := []string{"home", email}
    err = h.handler.Storage.CreateDir(userHomePath)
    if err != nil {
        fmt.Printf("DEBUG: Failed to create user home directory (non-fatal): %v\n", err)
    }

    // Create user's securestore directory for application data
    secureStorePath := []string{"home", email, "securestore"}
    err = h.handler.Storage.CreateDir(secureStorePath)
    if err != nil {
        fmt.Printf("DEBUG: Failed to create securestore directory (non-fatal): %v\n", err)
    }

    fmt.Printf("DEBUG: Setting current user and completing registration\n")
    h.setCurrentUser(c, email)
    
    if c.GetHeader("Content-Type") == "application/json" {
        c.JSON(http.StatusOK, gin.H{
            "data": "success",
            "result": "ok",
            "message": "Registration successful",
        })
    } else {
        c.Redirect(http.StatusFound, "/browser")
    }
    
    fmt.Printf("DEBUG: Registration completed successfully for: %s\n", email)
}

func (h *AuthHandler) clearCurrentUser(c *gin.Context) {
    fmt.Printf("DEBUG: Clearing user cookies\n")
    c.SetCookie("user", "", -1, "/", "", false, true)
    c.SetCookie("session", "", -1, "/", "", false, true)
}

// HandlePasswordResetGet handles GET requests for password reset
func (h *AuthHandler) HandlePasswordResetGet(c *gin.Context) {
	user := c.Query("u")
	dongle := c.Query("d")

	if user == "" || dongle == "" {
		c.HTML(http.StatusBadRequest, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": user,
		})
		return
	}

	userDongle, err := h.service.GetUserDongle(user)
	if err != nil || userDongle != dongle {
		c.HTML(http.StatusBadRequest, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": user,
		})
		return
	}

	c.HTML(http.StatusOK, "pwreset.html", gin.H{
		"user":    nil,
		"reguser": user,
	})
}

// HandlePasswordResetPost handles POST requests for password reset
func (h *AuthHandler) HandlePasswordResetPost(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	exists, err := h.service.UserExists(req.Email)
	if err != nil || !exists {
		c.HTML(http.StatusBadRequest, "lostpassword-baduser.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	err = h.service.UpdatePassword(req.Email, req.Password)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	c.HTML(http.StatusOK, "pwreset-ok.html", gin.H{
		"user":    nil,
		"reguser": req.Email,
	})
}

// HandleLostPassword handles lost password requests
func (h *AuthHandler) HandleLostPassword(c *gin.Context) {
	if c.Request.Method == "GET" {
		c.HTML(http.StatusOK, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	var req struct {
		Email string `json:"email" form:"email"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	exists, err := h.service.UserExists(req.Email)
	if err != nil || !exists {
		c.HTML(http.StatusBadRequest, "lostpassword-baduser.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	// Generate dongle and send email
	dongle := h.generateRandomString(20)
	err = h.service.SetUserDongle(req.Email, dongle)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	// Send password reset email
	err = h.sendLostPasswordEmail(req.Email, dongle, c.Request.Host)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	c.HTML(http.StatusOK, "lostpassword-sentemail.html", gin.H{
		"user":    nil,
		"reguser": req.Email,
	})
}

func (h *AuthHandler) setCurrentUser(c *gin.Context, user string) {
    fmt.Printf("DEBUG: Setting current user: '%s'\n", user)
    
    // Store email directly as cookie value
    c.SetSameSite(http.SameSiteStrictMode)
    c.SetCookie("user", user, 3600*24, "/", "", false, true)
    
    fmt.Printf("DEBUG: User cookie set successfully\n")
}

func (h *AuthHandler) generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

func (h *AuthHandler) sendLostPasswordEmail(userEmail, dongle, host string) error {
	// This would need the email service to be implemented
	// For now, we'll return nil
	link := fmt.Sprintf("http://%s/pwreset?u=%s&d=%s", host, userEmail, dongle)
	message := email.NewMessage()
	message.Subject = "Reset Password"
	message.BodyText = fmt.Sprintf("Please click the following link to reset password for user %s\n%s", userEmail, link)

	// Note: This assumes we have access to the email service and from email
	// We would need to implement this properly with the actual email service
	return nil
}

func (h *AuthHandler) HandleLoginGet(c *gin.Context) {
    c.HTML(http.StatusOK, "login.html", gin.H{
        "user": nil,
        "error": "",
    })
}

func (h *AuthHandler) HandleRegisterGet(c *gin.Context) {
    c.HTML(http.StatusOK, "register.html", gin.H{
        "user": nil,
        "error": "",
    })
}

// Update getCurrentUser with debugging
func (h *AuthHandler) getCurrentUser(c *gin.Context) string {
    userCookie, err := c.Cookie("user")
    if err != nil {
        return ""
    }

    // Handle both JSON format and plain text format
    if len(userCookie) > 0 && userCookie[0] == '"' && userCookie[len(userCookie)-1] == '"' {
        // JSON format
        var user string
        err = json.Unmarshal([]byte(userCookie), &user)
        if err != nil {
            return ""
        }
        return user
    }
    
    // Plain text format
    return userCookie
}
