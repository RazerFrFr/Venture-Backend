package routes

import (
	"net/http"
	"strconv"
	"time"

	"VentureBackend/utils"

	"github.com/gin-gonic/gin"
)

func RegisterTimelineRoutes(router *gin.Engine) {
	router.GET("/fortnite/api/calendar/v1/timeline", GetTimeline)
}

func GetTimeline(c *gin.Context) {
	memory := utils.GetVersionInfo(c.Request)

	todayAtMidnight := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour)
	todayOneMinuteBeforeMidnight := todayAtMidnight.Add(-1 * time.Minute)
	isoDate := todayOneMinuteBeforeMidnight.Format(time.RFC3339)

	seasonStr := strconv.Itoa(memory.Season)

	activeEvents := []map[string]string{
		{
			"eventType":   "EventFlag.Season" + seasonStr,
			"activeUntil": "9999-01-01T00:00:00.000Z",
			"activeSince": "2020-01-01T00:00:00.000Z",
		},
		{
			"eventType":   "EventFlag." + memory.Lobby,
			"activeUntil": "9999-01-01T00:00:00.000Z",
			"activeSince": "2020-01-01T00:00:00.000Z",
		},
	}

	stateTemplate := map[string]interface{}{
		"activeStorefronts":  []interface{}{},
		"eventNamedWeights":  map[string]interface{}{},
		"seasonNumber":       memory.Season,
		"seasonTemplateId":   "AthenaSeason:athenaseason" + seasonStr,
		"matchXpBonusPoints": 0,
		"seasonBegin":        "2020-01-01T13:00:00Z",
		"seasonEnd":          "9999-01-01T14:00:00Z",
		"seasonDisplayedEnd": "9999-01-01T07:30:00Z",
		"weeklyStoreEnd":     isoDate,
		"sectionStoreEnds":   map[string]string{"Featured": isoDate},
		"dailyStoreEnd":      isoDate,
	}

	states := []map[string]interface{}{
		{
			"validFrom":    "0001-01-01T00:00:00.000Z",
			"activeEvents": activeEvents,
			"state":        stateTemplate,
		},
	}

	resp := map[string]interface{}{
		"channels": map[string]interface{}{
			"client-matchmaking": map[string]interface{}{
				"states":      []interface{}{},
				"cacheExpire": "9999-01-01T00:00:00.000Z",
			},
			"client-events": map[string]interface{}{
				"states":      states,
				"cacheExpire": "9999-01-01T00:00:00.000Z",
			},
		},
		"eventsTimeOffsetHrs": 0,
		"cacheIntervalMins":   10,
		"currentTime":         time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, resp)
}
