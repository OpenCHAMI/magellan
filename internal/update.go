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

	"github.com/bikeshack/magellan/internal/log"
	"github.com/bikeshack/magellan/internal/util"
	bmclib "github.com/bmc-toolbox/bmclib/v2"
	"github.com/bmc-toolbox/bmclib/v2/constants"
	bmclibErrs "github.com/bmc-toolbox/bmclib/v2/errors"
	"github.com/sirupsen/logrus"
)


type UpdateParams struct {
	QueryParams
	FirmwarePath string
	FirmwareVersion string
	Component string
}

func UpdateFirmware(client *bmclib.Client, l *log.Logger, q *UpdateParams) error {
	// open BMC session and update driver registry
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.Timeout))
	client.Registry.FilterForCompatible(ctx)
	err := client.Open(ctx)
	if err != nil {
		ctxCancel()
		return fmt.Errorf("could not connect to bmc: %v", err)
	}

	defer client.Close(ctx)

	file, err := os.Open(q.FirmwarePath)
	if err != nil {
		ctxCancel()
		return fmt.Errorf("could not open firmware path: %v", err)
	}

	defer file.Close()

	taskId, err := client.FirmwareInstall(ctx, q.Component, constants.FirmwareApplyOnReset, true, file)
	if err != nil {
		ctxCancel()
		return fmt.Errorf("could not install firmware: %v", err)
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

func UpdateFirmwareV2(serverIP string, imageURI string, component string, q *QueryParams) error {
	url := baseUrl(q) + "UpdateService/Actions/SimpleUpdate"
	b := map[string]any{
		"UpdateComponent": component, // BMC, BIOS
		"TransferProtocol": "HTTP",
		"ImageURI": "http://" + serverIP + "/" +  imageURI,
	}
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("could not marshal data: %v", err)
	}
	headers := map[string]string{
		"Content-Type": "application/json",
		"cache-control": "no-cache",
	}
	res, _, err := util.MakeRequest(url, "POST", data, headers)
	if err != nil {
		return fmt.Errorf("something went wrong: %v", err)
	} else if res == nil {
		return fmt.Errorf("no response returned (url: %s)", url)
	} else if res.StatusCode != http.StatusOK {
		return fmt.Errorf("returned status code %d", res.StatusCode)
	}
	return nil
}