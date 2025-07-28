package profiles

import (
	"VentureBackend/static/models"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ProfileMap map[string]interface{}

func CreateProfiles(accountId string) (ProfileMap, error) {
	profiles := make(ProfileMap)
	profileFolder := "./static/profiles"

	files, err := os.ReadDir(profileFolder)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || strings.ToLower(file.Name()) == "allathena.json" {
			continue
		}

		filePath := filepath.Join(profileFolder, file.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var profile map[string]interface{}
		if err := json.Unmarshal(data, &profile); err != nil {
			return nil, err
		}

		profile["accountId"] = accountId
		profile["created"] = time.Now().UTC().Format(time.RFC3339)
		profile["updated"] = time.Now().UTC().Format(time.RFC3339)

		profileID, ok := profile["profileId"].(string)
		if !ok {
			continue
		}

		profiles[profileID] = profile
	}

	return profiles, nil
}

func ValidateProfile(profileId string, profiles *models.Profiles) bool {
	if profiles == nil || profiles.Profiles == nil {
		return false
	}
	_, exists := profiles.Profiles[profileId]
	return exists
}
