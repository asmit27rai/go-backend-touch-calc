package handlers

import (
    "encoding/json"
    "fmt"
    mt "math/rand"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
)

type WebAppHandler struct {
    handler *Handler
}

func NewWebAppHandler(h *Handler) *WebAppHandler {
    return &WebAppHandler{
        handler: h,
    }
}

type WebAppRequest struct {
    Action  string `json:"action" form:"action"`
    AppName string `json:"appname" form:"appname"`
    FName   string `json:"fname" form:"fname"`
    Data    string `json:"data" form:"data"`
    Content string `json:"content" form:"content"`
}

func (h *WebAppHandler) HandleWebApp(c *gin.Context) {
    var req WebAppRequest
    if err := c.ShouldBind(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "error",
            "result": "fail",
        })
        return
    }

    // Get current user from cookie
    user := h.getCurrentUser(c)
    if user == "" {
        c.JSON(http.StatusUnauthorized, gin.H{
            "data":   "usererror",
            "result": "fail",
        })
        return
    }

    // Log the action for debugging
    fmt.Printf("DEBUG: WebApp action: %s, user: %s, app: %s, file: %s\n", 
        req.Action, user, req.AppName, req.FName)

    switch req.Action {
    case "savefile":
        h.handleSaveFile(c, user, req)
    case "getfile":
        h.handleGetFile(c, user, req)
    case "delete-file":
        h.handleDeleteFile(c, user, req)
    case "listdir":
        h.handleListDir(c, user, req)
    case "save-multiple":
        h.handleSaveMultiple(c, user, req)
    case "get-data":
        h.handleGetData(c, user, req)
    case "backup":
        h.handleBackup(c, user, req)
    case "restore":
        h.handleRestore(c, user, req)
    case "save":
        h.handleSocialCalcSave(c, user, req)
    case "load":
        h.handleSocialCalcLoad(c, user, req)
    default:
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid action: " + req.Action,
            "result": "fail",
        })
    }
}

func (h *WebAppHandler) handleSaveFile(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or fname)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Saving file %s for user %s in app %s\n", req.FName, user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName, req.FName}
    // dirPath := []string{"home", user, "securestore", req.AppName}

    // Ensure entire directory structure exists
    err := h.ensureDirectoryStructure(user, req.AppName)
    if err != nil {
        fmt.Printf("DEBUG: Error ensuring directory structure: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create directory structure: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Save the data (include metadata for better debugging)
    fileData := map[string]interface{}{
        "content": req.Data,
        "user": user,
        "app": req.AppName,
        "filename": req.FName,
        "timestamp": fmt.Sprintf("%d", getCurrentTimestamp()),
        "storage_backend": h.handler.Config.StorageBackend,
    }

    dataJSON, err := json.Marshal(fileData)
    if err != nil {
        fmt.Printf("DEBUG: Error marshaling file data: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to encode file data",
            "result": "fail",
        })
        return
    }

    // Check if file exists
    _, err = h.handler.Storage.GetFile(path)
    if err != nil {
        // File doesn't exist, create it
        fmt.Printf("DEBUG: Creating new file: %s\n", req.FName)
        err = h.handler.Storage.CreateFile(path, string(dataJSON))
    } else {
        // File exists, update it
        fmt.Printf("DEBUG: Updating existing file: %s\n", req.FName)
        err = h.handler.Storage.UpdateFile(path, string(dataJSON))
    }

    if err != nil {
        fmt.Printf("DEBUG: Error saving file: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to save file: " + err.Error(),
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: File saved successfully: %s\n", req.FName)
    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
        "timestamp": getCurrentTimestamp(),
    })
}

func (h *WebAppHandler) handleGetFile(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or fname)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Getting file %s for user %s in app %s\n", req.FName, user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName, req.FName}
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        fmt.Printf("DEBUG: File not found: %s, error: %v\n", req.FName, err)
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "file not found: " + req.FName,
            "result": "fail",
        })
        return
    }

    // Handle both old format (direct string) and new format (JSON with metadata)
    var fileContent string
    if dataStr, ok := item.Data.(string); ok {
        // Try to parse as JSON first
        var fileData map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
            // New format with metadata
            if content, exists := fileData["content"]; exists {
                if contentStr, ok := content.(string); ok {
                    fileContent = contentStr
                } else {
                    fileContent = dataStr // Fallback to raw data
                }
            } else {
                fileContent = dataStr // No content field, use raw data
            }
        } else {
            // Old format, direct string
            fileContent = dataStr
        }
    } else {
        // Data is not a string, convert to JSON
        dataBytes, err := json.Marshal(item.Data)
        if err != nil {
            fmt.Printf("DEBUG: Error marshaling item data: %v\n", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "data":   "failed to read file data",
                "result": "fail",
            })
            return
        }
        fileContent = string(dataBytes)
    }

    fmt.Printf("DEBUG: File retrieved successfully: %s\n", req.FName)
    c.JSON(http.StatusOK, gin.H{
        "data":   fileContent,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleDeleteFile(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or fname)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Deleting file %s for user %s in app %s\n", req.FName, user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName, req.FName}
    err := h.handler.Storage.DeleteFile(path)
    if err != nil {
        fmt.Printf("DEBUG: Error deleting file: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to delete file: " + err.Error(),
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: File deleted successfully: %s\n", req.FName)
    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleListDir(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing app name",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Listing directory for user %s in app %s\n", user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName}
    
    // Ensure directory exists
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        // Directory doesn't exist, create it and return empty list
        err = h.ensureDirectoryStructure(user, req.AppName)
        if err != nil {
            fmt.Printf("DEBUG: Error creating directory: %v\n", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "data":   "failed to create directory: " + err.Error(),
                "result": "fail",
            })
            return
        }
        c.JSON(http.StatusOK, gin.H{
            "data":   []string{},
            "result": "ok",
            "storage_backend": h.handler.Config.StorageBackend,
        })
        return
    }

    // Extract file names from directory data
    var fileNames []string
    if data, ok := item.Data.([]interface{}); ok {
        for _, file := range data {
            if str, ok := file.(string); ok {
                fileNames = append(fileNames, str)
            }
        }
    }

    fmt.Printf("DEBUG: Directory listing successful, found %d files\n", len(fileNames))
    c.JSON(http.StatusOK, gin.H{
        "data":   fileNames,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleSaveMultiple(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.Content == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or content)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Saving multiple files for user %s in app %s\n", user, req.AppName)

    // Parse the content as JSON
    var filesData map[string]interface{}
    err := json.Unmarshal([]byte(req.Content), &filesData)
    if err != nil {
        fmt.Printf("DEBUG: Error parsing content JSON: %v\n", err)
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid JSON content: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Ensure directory structure exists
    err = h.ensureDirectoryStructure(user, req.AppName)
    if err != nil {
        fmt.Printf("DEBUG: Error ensuring directory structure: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create directory: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Save each file
    savedFiles := []string{}
    for filename, content := range filesData {
        if content == nil {
            continue
        }

        path := []string{"home", user, "securestore", req.AppName, filename}
        
        // Create file data with metadata
        fileData := map[string]interface{}{
            "content": content,
            "user": user,
            "app": req.AppName,
            "filename": filename,
            "timestamp": fmt.Sprintf("%d", getCurrentTimestamp()),
            "storage_backend": h.handler.Config.StorageBackend,
        }

        contentStr, err := json.Marshal(fileData)
        if err != nil {
            fmt.Printf("DEBUG: Error marshaling file data for %s: %v\n", filename, err)
            continue
        }

        // Check if file exists
        _, err = h.handler.Storage.GetFile(path)
        if err != nil {
            // File doesn't exist, create it
            err = h.handler.Storage.CreateFile(path, string(contentStr))
        } else {
            // File exists, update it
            err = h.handler.Storage.UpdateFile(path, string(contentStr))
        }

        if err != nil {
            fmt.Printf("DEBUG: Error saving file %s: %v\n", filename, err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "data":   "failed to save file: " + filename + " - " + err.Error(),
                "result": "fail",
            })
            return
        }
        
        savedFiles = append(savedFiles, filename)
    }

    fmt.Printf("DEBUG: Successfully saved %d files\n", len(savedFiles))
    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "saved_files": savedFiles,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleGetData(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.Content == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or content)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Getting multiple files for user %s in app %s\n", user, req.AppName)

    // Parse the content as JSON array of filenames
    var filenames []string
    err := json.Unmarshal([]byte(req.Content), &filenames)
    if err != nil {
        fmt.Printf("DEBUG: Error parsing filenames JSON: %v\n", err)
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid JSON content: " + err.Error(),
            "result": "fail",
        })
        return
    }

    data := make(map[string]interface{})
    retrievedCount := 0

    for _, filename := range filenames {
        path := []string{"home", user, "securestore", req.AppName, filename}
        item, err := h.handler.Storage.GetFile(path)
        if err == nil && item != nil {
            // Handle both old and new format
            if dataStr, ok := item.Data.(string); ok {
                var fileData map[string]interface{}
                if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
                    // New format with metadata
                    if content, exists := fileData["content"]; exists {
                        data[filename] = content
                    } else {
                        data[filename] = dataStr
                    }
                } else {
                    // Old format, direct string
                    data[filename] = dataStr
                }
            } else {
                data[filename] = item.Data
            }
            retrievedCount++
        } else {
            fmt.Printf("DEBUG: File not found: %s\n", filename)
        }
    }

    fmt.Printf("DEBUG: Retrieved %d out of %d requested files\n", retrievedCount, len(filenames))
    c.JSON(http.StatusOK, gin.H{
        "data":   data,
        "result": "ok",
        "retrieved_count": retrievedCount,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleBackup(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing app name",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Creating backup for user %s in app %s\n", user, req.AppName)

    // List all files in the app directory
    path := []string{"home", user, "securestore", req.AppName}
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "app directory not found",
            "result": "fail",
        })
        return
    }

    // Get all file contents
    backup := make(map[string]interface{})
    if data, ok := item.Data.([]interface{}); ok {
        for _, file := range data {
            if filename, ok := file.(string); ok {
                filePath := []string{"home", user, "securestore", req.AppName, filename}
                fileItem, err := h.handler.Storage.GetFile(filePath)
                if err == nil && fileItem != nil {
                    backup[filename] = fileItem.Data
                }
            }
        }
    }

    // Save backup with timestamp
    backupFilename := fmt.Sprintf("backup_%d.json", getCurrentTimestamp())
    backupPath := []string{"home", user, "securestore", req.AppName, backupFilename}
    
    backupData, err := json.Marshal(backup)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create backup data",
            "result": "fail",
        })
        return
    }

    err = h.handler.Storage.CreateFile(backupPath, string(backupData))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to save backup",
            "result": "fail",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "backup_file": backupFilename,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleRestore(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or backup filename)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Restoring backup %s for user %s in app %s\n", req.FName, user, req.AppName)

    // Get backup file
    backupPath := []string{"home", user, "securestore", req.AppName, req.FName}
    backupItem, err := h.handler.Storage.GetFile(backupPath)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "backup file not found",
            "result": "fail",
        })
        return
    }

    // Parse backup data
    var backupData map[string]interface{}
    if dataStr, ok := backupItem.Data.(string); ok {
        err = json.Unmarshal([]byte(dataStr), &backupData)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "data":   "invalid backup file format",
                "result": "fail",
            })
            return
        }
    } else {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid backup file data",
            "result": "fail",
        })
        return
    }

    // Restore files
    restoredCount := 0
    for filename, content := range backupData {
        path := []string{"home", user, "securestore", req.AppName, filename}
        contentStr, _ := json.Marshal(content)
        
        err = h.handler.Storage.UpdateFile(path, string(contentStr))
        if err == nil {
            restoredCount++
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "restored_files": restoredCount,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) ensureDirectoryStructure(user, appName string) error {
    // Create home directory
    homeDir := []string{"home"}
    _, err := h.handler.Storage.GetFile(homeDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(homeDir)
        if err != nil {
            return fmt.Errorf("failed to create home directory: %w", err)
        }
    }

    // Create user directory
    userDir := []string{"home", user}
    _, err = h.handler.Storage.GetFile(userDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(userDir)
        if err != nil {
            return fmt.Errorf("failed to create user directory: %w", err)
        }
    }

    // Create securestore directory
    secureDir := []string{"home", user, "securestore"}
    _, err = h.handler.Storage.GetFile(secureDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(secureDir)
        if err != nil {
            return fmt.Errorf("failed to create securestore directory: %w", err)
        }
    }

    // Create app directory
    appDir := []string{"home", user, "securestore", appName}
    _, err = h.handler.Storage.GetFile(appDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(appDir)
        if err != nil {
            return fmt.Errorf("failed to create app directory: %w", err)
        }
    }

    return nil
}

func getCurrentTimestamp() int64 {
    return 1691506800 // Mock timestamp for now
}

func (h *WebAppHandler) getCurrentUser(c *gin.Context) string {
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

// handleSocialCalcSave handles save requests from SocialCalc spreadsheet
func (h *WebAppHandler) handleSocialCalcSave(c *gin.Context, user string, req WebAppRequest) {
    // Get additional parameters that SocialCalc sends
    filename := c.PostForm("filename")
    content := c.PostForm("content")
    sessionid := c.PostForm("sessionid")
    
    // Use req fields as backup if form params are empty
    if filename == "" {
        filename = req.FName
    }
    if content == "" {
        content = req.Content
    }
    
    fmt.Printf("DEBUG: SocialCalc save - filename: %s, user: %s, sessionid: %s\n", 
        filename, user, sessionid)

    if filename == "" || content == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing filename or content",
            "result": "fail",
        })
        return
    }

    // Validate session if provided
    if sessionid != "" {
        session, exists := h.handler.Session.Get(sessionid)
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{
                "data":   "invalid session",
                "result": "fail",
            })
            return
        }
        
        // Double check user from session
        sessionUser, _ := session.GetString("user")
        if sessionUser != "" && sessionUser != user {
            c.JSON(http.StatusUnauthorized, gin.H{
                "data":   "session user mismatch",
                "result": "fail",
            })
            return
        }
    }

    // Use "touchcalc" as the app name for SocialCalc saves
    appName := "touchcalc"
    
    // Ensure directory structure exists
    err := h.ensureDirectoryStructure(user, appName)
    if err != nil {
        fmt.Printf("DEBUG: Error ensuring directory structure: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create directory structure: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Create file path
    path := []string{"home", user, "securestore", appName, filename + ".msc"}
    
    // Create file data with metadata (compatible with your existing format)
    fileData := map[string]interface{}{
        "content": content,
        "user": user,
        "app": appName,
        "filename": filename,
        "timestamp": fmt.Sprintf("%d", getCurrentTimestamp()),
        "storage_backend": h.handler.Config.StorageBackend,
        "type": "socialcalc_spreadsheet",
    }

    dataJSON, err := json.Marshal(fileData)
    if err != nil {
        fmt.Printf("DEBUG: Error marshaling file data: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to encode file data",
            "result": "fail",
        })
        return
    }

    // Check if file exists and save accordingly
    _, err = h.handler.Storage.GetFile(path)
    if err != nil {
        // File doesn't exist, create it
        fmt.Printf("DEBUG: Creating new SocialCalc file: %s\n", filename)
        err = h.handler.Storage.CreateFile(path, string(dataJSON))
    } else {
        // File exists, update it
        fmt.Printf("DEBUG: Updating existing SocialCalc file: %s\n", filename)
        err = h.handler.Storage.UpdateFile(path, string(dataJSON))
    }

    if err != nil {
        fmt.Printf("DEBUG: Error saving SocialCalc file: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to save file: " + err.Error(),
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: SocialCalc file saved successfully: %s\n", filename)
    
    // Return success response in format SocialCalc expects
    c.JSON(http.StatusOK, gin.H{
        "message": "File saved successfully",
        "filename": filename,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
        "timestamp": getCurrentTimestamp(),
    })
}

// handleSocialCalcLoad handles load requests from SocialCalc spreadsheet  
func (h *WebAppHandler) handleSocialCalcLoad(c *gin.Context, user string, req WebAppRequest) {
    filename := c.PostForm("filename")
    if filename == "" {
        filename = req.FName
    }
    
    if filename == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing filename",
            "result": "fail",
        })
        return
    }

    appName := "touchcalc"
    path := []string{"home", user, "securestore", appName, filename + ".msc"}
    
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        fmt.Printf("DEBUG: SocialCalc file not found: %s, error: %v\n", filename, err)
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "file not found: " + filename,
            "result": "fail",
        })
        return
    }

    // Extract content from stored data
    var fileContent string
    if dataStr, ok := item.Data.(string); ok {
        var fileData map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
            if content, exists := fileData["content"]; exists {
                if contentStr, ok := content.(string); ok {
                    fileContent = contentStr
                } else {
                    fileContent = dataStr
                }
            } else {
                fileContent = dataStr
            }
        } else {
            fileContent = dataStr
        }
    } else {
        dataBytes, _ := json.Marshal(item.Data)
        fileContent = string(dataBytes)
    }

    fmt.Printf("DEBUG: SocialCalc file loaded successfully: %s\n", filename)
    c.JSON(http.StatusOK, gin.H{
        "data":   fileContent,
        "filename": filename,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) HandleSave(c *gin.Context) {
	if c.Request.Method == "GET" {
		h.handleSaveGet(c)
	} else {
		h.handleSavePost(c)
	}
}

func (h *WebAppHandler) handleSaveGet(c *gin.Context) {
	user := h.getCurrentUser(c)
	if user == "" {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	fmt.Printf("DEBUG: Loading file list for user: %s\n", user)

	// Get user's files from storage
	path := []string{"home", user}
	item, err := h.handler.Storage.GetFile(path)
	var entries []map[string]interface{}
	
	if err != nil || item == nil {
		fmt.Printf("DEBUG: User directory not found, creating structure\n")
		// Create user directory if it doesn't exist
		err = h.handler.Storage.CreateDir(path)
		if err != nil {
			fmt.Printf("DEBUG: Failed to create user directory: %v\n", err)
		}
		
		// Create default file
		defaultPath := []string{"home", user, "default"}
		defaultData := map[string]interface{}{
			"user":  user,
			"fname": "default",
			"data":  "A1:Welcome to TouchCalc\nB1:Hello " + user + "\nA2:Start editing here\nB2:Your data auto-saves\n\n",
		}
		dataJSON, _ := json.Marshal(defaultData)
		h.handler.Storage.CreateFile(defaultPath, string(dataJSON))
		
		entries = []map[string]interface{}{
			{"fname": "default"},
		}
	} else {
		// Extract file names from directory
		if data, ok := item.Data.([]interface{}); ok {
			for _, file := range data {
				if str, ok := file.(string); ok {
					entries = append(entries, map[string]interface{}{
						"fname": str,
					})
				}
			}
		}
	}

	fmt.Printf("DEBUG: Found %d files for user %s\n", len(entries), user)

	c.HTML(http.StatusOK, "allusersheets.html", gin.H{
		"entries": entries,
		"user":    user,
	})
}

func (h *WebAppHandler) handleSavePost(c *gin.Context) {
	user := h.getCurrentUser(c)
	if user == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"result": "fail",
			"data":   "usererror",
		})
		return
	}

	fname := c.PostForm("fname")
	data := c.PostForm("data")
	
	fmt.Printf("DEBUG: Saving file %s for user %s\n", fname, user)
	
	if fname == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result": "fail",
			"data":   "missing filename",
		})
		return
	}

	path := []string{"home", user, fname}
	
	// Create file data with metadata
	fileData := map[string]interface{}{
		"user":     user,
		"fname":    fname,
		"data":     data,
		"timestamp": time.Now().Unix(),
	}
	dataJSON, _ := json.Marshal(fileData)
	
	// Check if file exists
	_, err := h.handler.Storage.GetFile(path)
	if err != nil {
		// Create new file
		err = h.handler.Storage.CreateFile(path, string(dataJSON))
	} else {
		// Update existing file
		err = h.handler.Storage.UpdateFile(path, string(dataJSON))
	}

	if err != nil {
		fmt.Printf("DEBUG: Error saving file: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"result": "fail",
			"data":   "failed to save file",
		})
		return
	}

	fmt.Printf("DEBUG: File %s saved successfully\n", fname)
	c.JSON(http.StatusOK, gin.H{
		"result": "ok",
		"data":   "Done",
	})
}

// HandleUserSheet handles the /usersheet endpoint
func (h *WebAppHandler) HandleUserSheet(c *gin.Context) {
	user := h.getCurrentUser(c)
	if user == "" {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	fname := c.PostForm("pagename")
	deleteFlag := c.PostForm("delete")
	
	fmt.Printf("DEBUG: UserSheet request - user: %s, file: %s, delete: %s\n", user, fname, deleteFlag)
	
	if fname == "" {
		c.Redirect(http.StatusFound, "/save")
		return
	}

	path := []string{"home", user, fname}

	// Handle delete operation
	if deleteFlag == "yes" {
		fmt.Printf("DEBUG: Deleting file %s for user %s\n", fname, user)
		err := h.handler.Storage.DeleteFile(path)
		if err != nil {
			fmt.Printf("DEBUG: Failed to delete file: %v\n", err)
		}
		c.Redirect(http.StatusFound, "/save")
		return
	}

	// Get file for editing
	item, err := h.handler.Storage.GetFile(path)
	if err != nil {
		fmt.Printf("DEBUG: File %s not found for user %s\n", fname, user)
		c.Redirect(http.StatusFound, "/save")
		return
	}

	// Generate session ID
	sessionID := h.generateRandomString(6)
	
	// Extract content if it's in JSON format
	var content string
	if dataStr, ok := item.Data.(string); ok {
		var fileData map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
			if contentField, exists := fileData["data"]; exists {
				if contentStr, ok := contentField.(string); ok {
					content = contentStr
				} else {
					content = dataStr
				}
			} else if contentField, exists := fileData["content"]; exists {
				if contentStr, ok := contentField.(string); ok {
					content = contentStr
				} else {
					content = dataStr
				}
			} else {
				content = dataStr
			}
		} else {
			content = dataStr
		}
	} else {
		contentBytes, _ := json.Marshal(item.Data)
		content = string(contentBytes)
	}
	
	entry := map[string]interface{}{
		"fname":        fname,
		"sheetstr":     content,
		"sheetmscestr": "",
		"session":      sessionID,
	}

	fmt.Printf("DEBUG: Opening file %s for editing\n", fname)
	c.HTML(http.StatusOK, "importcollabload.html", gin.H{
		"entry": entry,
		"user":  user,
	})
}

// HandleImportGet handles GET requests to /import
func (h *WebAppHandler) HandleImportGet(c *gin.Context) {
	session := h.generateRandomString(6)
	
	c.SetCookie("session", session, 3600, "/", "", false, true)
	c.SetCookie("idinsession", "1", 3600, "/", "", false, true)
	
	fmt.Printf("DEBUG: Import page loaded with session: %s\n", session)
	
	c.HTML(http.StatusOK, "importcollab.html", gin.H{
		"entry": map[string]interface{}{
			"fname":        "test",
			"sheetstr":     "",
			"sheetmscestr": "",
			"session":      session,
		},
	})
}

// HandleImportPost handles POST requests to /import
func (h *WebAppHandler) HandleImportPost(c *gin.Context) {
	session, _ := c.Cookie("session")
	user := h.getCurrentUser(c)
	
	fmt.Printf("DEBUG: Import POST request - session: %s, user: %s\n", session, user)
	
	file, err := c.FormFile("upload")
	if err != nil {
		fmt.Printf("DEBUG: No file uploaded: %v\n", err)
		c.HTML(http.StatusBadRequest, "importerror.html", gin.H{
			"error": "No file uploaded",
		})
		return
	}

	fname := file.Filename
	fmt.Printf("DEBUG: Processing uploaded file: %s\n", fname)
	
	// Open and read file
	src, err := file.Open()
	if err != nil {
		fmt.Printf("DEBUG: Failed to read file: %v\n", err)
		c.HTML(http.StatusInternalServerError, "importerror.html", gin.H{
			"error": "Failed to read file",
		})
		return
	}
	defer src.Close()

	// Read file contents
	content := make([]byte, file.Size)
	src.Read(content)
	
	var wbook string
	
	// Handle different file types
	if strings.HasSuffix(strings.ToLower(fname), ".msc") || strings.HasSuffix(strings.ToLower(fname), ".msce") {
		wbook = string(content)
	} else {
		// For other file types, treat as plain text for now
		// In a real implementation, you'd convert Excel/CSV files here
		wbook = string(content)
	}

	// If user is logged in, save the imported file
	if user != "" {
		// Remove file extension for storage
		baseName := fname
		if idx := strings.LastIndex(fname, "."); idx != -1 {
			baseName = fname[:idx]
		}
		
		path := []string{"home", user, baseName}
		fileData := map[string]interface{}{
			"user":      user,
			"fname":     baseName,
			"data":      wbook,
			"imported":  true,
			"timestamp": time.Now().Unix(),
		}
		dataJSON, _ := json.Marshal(fileData)
		h.handler.Storage.CreateFile(path, string(dataJSON))
		
		fmt.Printf("DEBUG: Imported file saved as %s for user %s\n", baseName, user)
	}

	c.HTML(http.StatusOK, "importcollabload.html", gin.H{
		"entry": map[string]interface{}{
			"fname":        fname,
			"sheetmscestr": wbook,
			"sheetstr":     wbook,
			"session":      session,
		},
		"user": user,
	})
}

// HandleDownloadFile handles file download requests
func (h *WebAppHandler) HandleDownloadFile(c *gin.Context) {
	user := h.getCurrentUser(c)
	if user == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"result": "fail",
			"data":   "usererror",
		})
		return
	}

	fname := c.PostForm("fname")
	format := c.PostForm("format")
	
	fmt.Printf("DEBUG: Download request - user: %s, file: %s, format: %s\n", user, fname, format)
	
	if fname == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result": "fail",
			"data":   "missing filename",
		})
		return
	}

	path := []string{"home", user, fname}
	item, err := h.handler.Storage.GetFile(path)
	if err != nil {
		fmt.Printf("DEBUG: File not found for download: %s\n", fname)
		c.JSON(http.StatusNotFound, gin.H{
			"result": "fail",
			"data":   "file not found",
		})
		return
	}

	// Extract content
	var content string
	if dataStr, ok := item.Data.(string); ok {
		var fileData map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
			if dataField, exists := fileData["data"]; exists {
				if dataFieldStr, ok := dataField.(string); ok {
					content = dataFieldStr
				} else {
					content = dataStr
				}
			} else {
				content = dataStr
			}
		} else {
			content = dataStr
		}
	} else {
		dataBytes, _ := json.Marshal(item.Data)
		content = string(dataBytes)
	}

	// Set appropriate headers based on format
	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename="+fname+".csv")
	case "xlsx":
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename="+fname+".xlsx")
	case "msc":
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", "attachment; filename="+fname+".msc")
	default:
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", "attachment; filename="+fname)
	}

	c.String(http.StatusOK, content)
}

// HandleHTMLToPDFGet handles GET requests to /htmltopdf
func (h *WebAppHandler) HandleHTMLToPDFGet(c *gin.Context) {
	user := h.getCurrentUser(c)
	c.HTML(http.StatusOK, "htmltopdf.html", gin.H{
		"user": user,
	})
}

// HandleHTMLToPDFPost handles POST requests to /htmltopdf
func (h *WebAppHandler) HandleHTMLToPDFPost(c *gin.Context) {
	user := h.getCurrentUser(c)
	if user == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"result": "fail",
			"data":   "usererror",
		})
		return
	}

	htmlContent := c.PostForm("html")
	filename := c.PostForm("filename")
	
	fmt.Printf("DEBUG: PDF conversion request - user: %s, filename: %s\n", user, filename)
	
	if htmlContent == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result": "fail",
			"data":   "missing HTML content",
		})
		return
	}

	if filename == "" {
		filename = "document"
	}

	// Placeholder for PDF generation - implement with wkhtmltopdf or similar
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename="+filename+".pdf")
	c.String(http.StatusOK, "PDF conversion feature coming soon. HTML content length: %d", len(htmlContent))
}

// Helper method to generate random session IDs (add to existing methods)
func (h *WebAppHandler) generateRandomString(length int) string {
    const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, length)
    // Seed math/rand once (not cryptographically secure)
    mt.Seed(time.Now().UnixNano())
    for i := range b {
        b[i] = charset[mt.Intn(len(charset))]
    }
    return string(b)
}