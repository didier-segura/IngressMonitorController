package uptimerobot

import (
	"encoding/json"
	"errors"
	Http "net/http"
	"net/url"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	endpointmonitorv1alpha1 "github.com/stakater/IngressMonitorController/pkg/apis/endpointmonitor/v1alpha1"
	"github.com/stakater/IngressMonitorController/pkg/config"
	"github.com/stakater/IngressMonitorController/pkg/http"
	"github.com/stakater/IngressMonitorController/pkg/models"
)

type UpTimeMonitorService struct {
	apiKey            string
	url               string
	alertContacts     string
	statusPageService UpTimeStatusPageService
}

func (monitor *UpTimeMonitorService) Equal(oldMonitor models.Monitor, newMonitor models.Monitor) bool {
	// TODO: Retrieve oldMonitor config and compare it here
	return false
}

func (monitor *UpTimeMonitorService) Setup(p config.Provider) {
	monitor.apiKey = p.ApiKey
	monitor.url = p.ApiURL
	monitor.alertContacts = p.AlertContacts
	monitor.statusPageService = UpTimeStatusPageService{}
	monitor.statusPageService.Setup(p)
}

func (monitor *UpTimeMonitorService) GetByName(name string) (*models.Monitor, error) {
	action := "getMonitors"

	client := http.CreateHttpClient(monitor.url + action)

	body := "api_key=" + monitor.apiKey + "&format=json&logs=1&alert_contacts=1&search=" + name

	response := client.PostUrlEncodedFormBody(body)

	if response.StatusCode == Http.StatusOK {
		var f UptimeMonitorGetMonitorsResponse
		err := json.Unmarshal(response.Bytes, &f)
		if err != nil {
			return nil, err
		}

		if f.Monitors != nil {
			for _, monitor := range f.Monitors {
				if monitor.FriendlyName == name {
					return UptimeMonitorMonitorToBaseMonitorMapper(monitor), nil
				}
			}
		}

		return nil, nil
	}

	errorString := "GetByName Request failed for name: " + name + ". Status Code: " + strconv.Itoa(response.StatusCode)

	log.Println(errorString)
	return nil, errors.New(errorString)
}

func (monitor *UpTimeMonitorService) GetAllByName(name string) ([]models.Monitor, error) {
	action := "getMonitors"

	client := http.CreateHttpClient(monitor.url + action)

	body := "api_key=" + monitor.apiKey + "&format=json&logs=1" + "&search=" + name

	response := client.PostUrlEncodedFormBody(body)

	if response.StatusCode == 200 {
		var f UptimeMonitorGetMonitorsResponse
		err := json.Unmarshal(response.Bytes, &f)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		if len(f.Monitors) > 0 {
			return UptimeMonitorMonitorsToBaseMonitorsMapper(f.Monitors), nil
		}
		return nil, nil
	}

	errorString := "GetAllByName Request failed for name: " + name + ". Status Code: " + strconv.Itoa(response.StatusCode)

	log.Println(errorString)
	return nil, errors.New(errorString)
}

func (monitor *UpTimeMonitorService) GetAll() []models.Monitor {

	action := "getMonitors"

	client := http.CreateHttpClient(monitor.url + action)

	body := "api_key=" + monitor.apiKey + "&format=json&logs=1"

	response := client.PostUrlEncodedFormBody(body)

	if response.StatusCode == Http.StatusOK {

		var f UptimeMonitorGetMonitorsResponse
		err := json.Unmarshal(response.Bytes, &f)
		if err != nil {
			log.Error(err)
			return nil
		}

		return UptimeMonitorMonitorsToBaseMonitorsMapper(f.Monitors)

	}

	log.Println("GetAllMonitors Request failed. Status Code: " + strconv.Itoa(response.StatusCode))
	return nil

}

func (monitor *UpTimeMonitorService) Add(m models.Monitor) {
	action := "newMonitor"

	client := http.CreateHttpClient(monitor.url + action)

	body := monitor.processProviderConfig(m, true)

	response := client.PostUrlEncodedFormBody(body)

	if response.StatusCode == Http.StatusOK {
		var f UptimeMonitorNewMonitorResponse
		err := json.Unmarshal(response.Bytes, &f)
		if err != nil {
			log.Error(err, "Monitor couldn't be added: "+m.Name)
		}

		if f.Stat == "ok" {
			log.Println("Monitor Added: " + m.Name)
			monitor.handleStatusPagesConfig(m, strconv.Itoa(f.Monitor.ID))
		} else {
			log.Println("Monitor couldn't be added: " + m.Name + ". Error: " + f.Error.Message)
		}
	} else {
		log.Printf("AddMonitor Request failed. Status Code: " + strconv.Itoa(response.StatusCode))
	}
}

func (monitor *UpTimeMonitorService) Update(m models.Monitor) {
	action := "editMonitor"

	client := http.CreateHttpClient(monitor.url + action)

	body := monitor.processProviderConfig(m, false)

	response := client.PostUrlEncodedFormBody(body)

	if response.StatusCode == Http.StatusOK {
		var f UptimeMonitorStatusMonitorResponse
		err := json.Unmarshal(response.Bytes, &f)
		if err != nil {
			log.Error(err, "Monitor couldn't be updated: "+m.Name)
		}
		if f.Stat == "ok" {
			log.Println("Monitor Updated: " + m.Name)
			monitor.handleStatusPagesConfig(m, strconv.Itoa(f.Monitor.ID))
		} else {
			log.Println("Monitor couldn't be updated: " + m.Name + ". Error: " + f.Error.Message)
		}
	} else {
		log.Println("UpdateMonitor Request failed. Status Code: " + strconv.Itoa(response.StatusCode))
	}
}

func (monitor *UpTimeMonitorService) processProviderConfig(m models.Monitor, createMonitorRequest bool) string {
	var body string

	// if createFunction is true, generate query for create else for update
	if createMonitorRequest {
		body = "api_key=" + monitor.apiKey + "&format=json&url=" + url.QueryEscape(m.URL) + "&friendly_name=" + url.QueryEscape(m.Name)
	} else {
		body = "api_key=" + monitor.apiKey + "&format=json&id=" + m.ID + "&friendly_name=" + m.Name + "&url=" + m.URL
	}

	// Retrieve provider configuration
	providerConfig, _ := m.Config.(*endpointmonitorv1alpha1.UptimeRobotConfig)

	if providerConfig != nil && len(providerConfig.AlertContacts) != 0 {
		body += "&alert_contacts=" + providerConfig.AlertContacts
	} else {
		body += "&alert_contacts=" + monitor.alertContacts
	}

	if providerConfig != nil && providerConfig.Interval > 0 {
		body += "&interval=" + strconv.Itoa(providerConfig.Interval)
	}

	if providerConfig != nil && len(providerConfig.MaintenanceWindows) != 0 {
		body += "&mwindows=" + providerConfig.MaintenanceWindows
	}

	if providerConfig != nil && len(providerConfig.MonitorType) != 0 {
		if strings.Contains(strings.ToLower(providerConfig.MonitorType), "http") {
			body += "&type=1"
		} else if strings.Contains(strings.ToLower(providerConfig.MonitorType), "keyword") {
			body += "&type=2"

			if providerConfig != nil && len(providerConfig.KeywordExists) != 0 {

				if strings.Contains(strings.ToLower(providerConfig.KeywordExists), "yes") {
					body += "&keyword_type=1"
				} else if strings.Contains(strings.ToLower(providerConfig.KeywordExists), "no") {
					body += "&keyword_type=2"
				}

			} else {
				body += "&keyword_type=1" // By default 1 (check if keyword exists)
			}

			if providerConfig != nil && len(providerConfig.KeywordValue) != 0 {
				body += "&keyword_value=" + providerConfig.KeywordValue
			} else {
				log.Error("Monitor is of type Keyword but the `keyword-value` annotation is missing")
			}
		}
	} else {
		body += "&type=1" // By default monitor is of type HTTP
	}
	return body
}

func (monitor *UpTimeMonitorService) Remove(m models.Monitor) {
	action := "deleteMonitor"

	client := http.CreateHttpClient(monitor.url + action)

	log.Println(m.ID)
	body := "api_key=" + monitor.apiKey + "&format=json&id=" + m.ID

	response := client.PostUrlEncodedFormBody(body)

	if response.StatusCode == Http.StatusOK {
		var f UptimeMonitorStatusMonitorResponse
		err := json.Unmarshal(response.Bytes, &f)
		if err != nil {
			log.Error(err, "Monitor couldn't be removed: "+m.Name)
		}
		if f.Stat == "ok" {
			log.Println("Monitor Removed: " + m.Name)
		} else {
			log.Println("Monitor couldn't be removed: " + m.Name + ". Error: " + f.Error.Message)
			log.Println(string(body))
		}
	} else {
		log.Println("RemoveMonitor Request failed. Status Code: " + strconv.Itoa(response.StatusCode))
	}
}

func (monitor *UpTimeMonitorService) handleStatusPagesConfig(monitorToAdd models.Monitor, monitorId string) {
	// Retrieve provider configuration
	providerConfig, _ := monitorToAdd.Config.(*endpointmonitorv1alpha1.UptimeRobotConfig)

	if providerConfig != nil && len(providerConfig.StatusPages) != 0 {
		IDs := strings.Split(providerConfig.StatusPages, "-")
		for i := range IDs {
			monitor.updateStatusPages(IDs[i], models.Monitor{ID: monitorId})
		}
	}
}

func (monitor *UpTimeMonitorService) updateStatusPages(statusPages string, monitorToAdd models.Monitor) {
	statusPage := UpTimeStatusPage{ID: statusPages}
	_, err := monitor.statusPageService.AddMonitorToStatusPage(statusPage, monitorToAdd)
	if err != nil {
		log.Println("Monitor couldn't be added to status page: " + err.Error())
	}
}
