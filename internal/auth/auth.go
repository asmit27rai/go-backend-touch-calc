package auth

import (
	"fmt"
	"strings"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
)

const (
	UserDir     = "users"
	UserDirPath = "home/users"
)

type Service struct {
	storage storage.Storage
}

func NewService(storage storage.Storage) *Service {
	return &Service{
		storage: storage,
	}
}

func (s *Service) getUserPath(email string) []string {
	return []string{"home", UserDir, email}
}

func (s *Service) UserExists(email string) (bool, error) {
    path := s.getUserPath(email)
    item, err := s.storage.GetFile(path)
    if err != nil {
        if err == storage.ErrNotFound {
            return false, nil
        }
        return false, err
    }
    return item != nil, nil
}

func (s *Service) GetUser(email string) (*models.User, error) {
	path := s.getUserPath(email)
	item, err := s.storage.GetFile(path)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Convert the data to string and then parse as User
	dataStr, ok := item.Data.(string)
	if !ok {
		return nil, fmt.Errorf("invalid user data format")
	}

	user, err := models.UserFromJSON(dataStr)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) CreateUser(email, password string) error {
    // First check if user already exists
    exists, err := s.UserExists(email)
    if err != nil {
        return fmt.Errorf("error checking user existence: %w", err)
    }
    if exists {
        return fmt.Errorf("user already exists")
    }

    user, err := models.NewUser(email, password)
    if err != nil {
        return fmt.Errorf("error creating user model: %w", err)
    }

    // Ensure the root home directory exists
    homeDir := []string{"home"}
    err = s.storage.CreateDir(homeDir)
    if err != nil {
        return fmt.Errorf("error creating home directory: %w", err)
    }

    // Ensure parent directory exists
    parentPath := []string{"home", UserDir}
    err = s.storage.CreateDir(parentPath)
    if err != nil {
        return fmt.Errorf("error creating users directory: %w", err)
    }

    path := s.getUserPath(email)
    userData, err := user.ToJSON()
    if err != nil {
        return fmt.Errorf("error serializing user data: %w", err)
    }

    return s.storage.CreateFile(path, userData)
}

func (s *Service) AuthenticateUser(email, password string) (bool, error) {
	user, err := s.GetUser(email)
	if err != nil {
		return false, err
	}

	if !user.GetConfirmed() {
		return false, fmt.Errorf("user not confirmed")
	}

	return user.Authenticate(password), nil
}

func (s *Service) UpdatePassword(email, newPassword string) error {
	user, err := s.GetUser(email)
	if err != nil {
		return err
	}

	err = user.SetPassword(newPassword)
	if err != nil {
		return err
	}

	return s.setUser(user)
}

func (s *Service) SetUserDongle(email, dongle string) error {
	user, err := s.GetUser(email)
	if err != nil {
		return err
	}

	user.SetDongle(dongle)
	return s.setUser(user)
}

func (s *Service) GetUserDongle(email string) (string, error) {
	user, err := s.GetUser(email)
	if err != nil {
		return "", err
	}

	return user.GetDongle(), nil
}

func (s *Service) ConfirmUser(email string) error {
	user, err := s.GetUser(email)
	if err != nil {
		return err
	}

	user.SetConfirmed()
	return s.setUser(user)
}

func (s *Service) DeleteUser(email string) error {
	exists, err := s.UserExists(email)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("user does not exist")
	}

	path := s.getUserPath(email)
	return s.storage.DeleteFile(path)
}

func (s *Service) setUser(user *models.User) error {
	path := s.getUserPath(user.Email)
	userData, err := user.ToJSON()
	if err != nil {
		return err
	}

	return s.storage.UpdateFile(path, userData)
}

// ValidateEmail performs basic email validation
func ValidateEmail(email string) bool {
	return strings.Contains(email, "@") && len(email) > 3
}