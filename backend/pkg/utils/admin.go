package utils

import (
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
)

// IsAdmin checks if the user with given ID is an admin user
func IsAdmin(userID uint) bool {
	var user models.User
	err := database.DB.First(&user, userID).Error
	if err != nil {
		return false
	}
	return user.Username == "admin"
}

// HasPermissionOnProject checks if user has permission on a project (owner or admin)
func HasPermissionOnProject(userID uint, projectID uint) bool {
	if IsAdmin(userID) {
		return true
	}

	var project models.Project
	err := database.DB.Where("id = ? AND user_id = ? AND status = ?", projectID, userID, 1).First(&project).Error
	return err == nil
}

// HasPermissionOnTestCase checks if user has permission on a test case (owner, project owner, or admin)
func HasPermissionOnTestCase(userID uint, testCaseID uint) bool {
	if IsAdmin(userID) {
		return true
	}

	var testCase models.TestCase
	err := database.DB.Preload("Project").Where("id = ? AND status = ?", testCaseID, 1).First(&testCase).Error
	if err != nil {
		return false
	}

	return testCase.UserID == userID || testCase.Project.UserID == userID
}

// HasPermissionOnTestSuite checks if user has permission on a test suite (owner, project owner, or admin)
func HasPermissionOnTestSuite(userID uint, testSuiteID uint) bool {
	if IsAdmin(userID) {
		return true
	}

	var testSuite models.TestSuite
	err := database.DB.Preload("Project").Where("id = ? AND status = ?", testSuiteID, 1).First(&testSuite).Error
	if err != nil {
		return false
	}

	return testSuite.UserID == userID || testSuite.Project.UserID == userID
}
