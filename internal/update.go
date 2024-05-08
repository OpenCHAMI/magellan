package magellan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/OpenCHAMI/magellan/internal/log"
	"github.com/OpenCHAMI/magellan/internal/util"
	bmclib "github.com/bmc-toolbox/bmclib/v2"
	"github.com/bmc-toolbox/bmclib/v2/constants"
	bmclibErrs "github.com/bmc-toolbox/bmclib/v2/errors"
	"github.com/sirupsen/logrus"
)

type UpdateParams struct {
	QueryParams
	FirmwarePath     string
	FirmwareVersion  string
	Component        string
	TransferProtocol string
}

// NOTE: Does not work since OpenBMC, whic bmclib uses underneath, does not
// support multipart updates. See issue: https://github.com/bmc-toolbox/bmclib/issues/341
func UpdateFirmware(client *bmclib.Client, l *log.Logger, q *UpdateParams) error {
	if q.Component == "" {
		return fmt.Errorf("component is required")
	}

	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return fmt.Errorf("failed toconnect to bmc: %v", err)
	}

	defer client.Close(ctx)

	file, err := os.Open(q.FirmwarePath)
	if err != nil {
		ctxCancel()
		return fmt.Errorf("failed toopen firmware path: %v", err)
	}

	defer file.Close()

	taskId, err := client.FirmwareInstall(ctx, q.Component, constants.FirmwareApplyOnReset, true, file)
	if err != nil {
		ctxCancel()
		return fmt.Errorf("failed toinstall firmware: %v", err)
	}

	for {
		if ctx.Err() != nil {
			ctxCancel()
			return fmt.Errorf("context error: %v", ctx.Err())
		}

		state, err := client.FirmwareInstallStatus(ctx, q.FirmwareVersion, q.Component, taskId)
		if err != nil {
			// when its under update a connection refused is returned
			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "operation timed out") {
				l.Log.Info("BMC refused connection, BMC most likely resetting...")
				time.Sleep(2 * time.Second)

				continue
			}

			if errors.Is(err, bmclibErrs.ErrSessionExpired) || strings.Contains(err.Error(), "session expired") {
				err := client.Open(ctx)
				if err != nil {
					l.Log.Fatal(err, "bmc re-login failed")
				}

				l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("BMC session expired, logging in...")

				continue
			}

			l.Log.Fatal(err)
		}

		switch state {
		case constants.FirmwareInstallRunning, constants.FirmwareInstallInitializing:
			l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("firmware install running")

		case constants.FirmwareInstallFailed:
			ctxCancel()
			l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("firmware install failed")
			return fmt.Errorf("failed to install firmware")

		case constants.FirmwareInstallComplete:
			ctxCancel()
			l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("firmware install completed")
			return nil

		case constants.FirmwareInstallPowerCyleHost:
			l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("host powercycle required")

			if _, err := client.SetPowerState(ctx, "cycle"); err != nil {
				ctxCancel()
				l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("error power cycling host for install")
				return fmt.Errorf("failed to install firmware")
			}

			ctxCancel()
			l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("host power cycled, all done!")
			return nil
		default:
			l.Log.WithFields(logrus.Fields{"state": state, "component": q.Component}).Info("unknown state returned")
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}

func UpdateFirmwareRemote(q *UpdateParams) error {
	url := baseRedfishUrl(&q.QueryParams) + "/redfish/v1/UpdateService/Actions/SimpleUpdate"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"cache-control": "no-cache",
	}
	b := map[string]any{
		"UpdateComponent":  q.Component, // BMC, BIOS
		"TransferProtocol": q.TransferProtocol,
		"ImageURI":         q.FirmwarePath,
	}
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("failed tomarshal data: %v", err)
	}
	res, body, err := util.MakeRequest(nil, url, "POST", data, headers)
	if err != nil {
		return fmt.Errorf("something went wrong: %v", err)
	} else if res == nil {
		return fmt.Errorf("no response returned (url: %s)", url)
	}
	if len(body) > 0 {
		fmt.Printf("%d: %v\n", res.StatusCode, string(body))
	}
	return nil
}

func GetUpdateStatus(q *UpdateParams) error {
	url := baseRedfishUrl(&q.QueryParams) + "/redfish/v1/UpdateService"
	res, body, err := util.MakeRequest(nil, url, "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("something went wrong: %v", err)
	} else if res == nil {
		return fmt.Errorf("no response returned (url: %s)", url)
	} else if res.StatusCode != http.StatusOK {
		return fmt.Errorf("returned status code %d", res.StatusCode)
	}
	if len(body) > 0 {
		fmt.Printf("%v\n", string(body))
	}
	return nil
}

// func UpdateFirmwareLocal(q *UpdateParams) error {
// 	fwUrl := baseUrl(&q.QueryParams) + ""
// 	url := baseUrl(&q.QueryParams) + "UpdateService/Actions/"
// 	headers := map[string]string {

// 	}

// 	// get etag from FW inventory
// 	response, err := util.MakeRequest()

// 	// load file from disk
// 	file, err := os.ReadFile(q.FirmwarePath)
// 	if err != nil {
// 		return fmt.Errorf("failed toread file: %v", err)
// 	}

// 	switch q.TransferProtocol {
// 	case "HTTP":
// 	default:
// 		return fmt.Errorf("transfer protocol not supported")
// 	}
// 	return nil
// }
