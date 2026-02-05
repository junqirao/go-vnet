// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"crypto/rsa"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
)

type (
	IDevice interface {
		CreateDevice(ctx context.Context, deviceKey string, dev *entity.Device) (id string, err error)
		GetDeviceInfo(ctx context.Context, deviceId string) (dev *model.Device, err error)
		GetDeviceById(ctx context.Context, id string) (dev *entity.Device, err error)
		GetDeviceByName(ctx context.Context, name string) (dev *entity.Device, err error)
		PrivateKeyBySubDeviceId(ctx context.Context, id uint64, key string) (pri *rsa.PrivateKey, err error)
		VerifyByName(ctx context.Context, name string, key string, nonce string, signature string) (dev *entity.Device, err error)
		VerifyById(ctx context.Context, id string, key string, nonce string, signature string) (dev *entity.Device, err error)
		CreateSubDevice(ctx context.Context, deviceId string, networkId string, quota int, settings *model.SubDeviceSettings) (err error)
		GetSubDeviceById(ctx context.Context, id uint64) (sub *model.SubDevice, err error)
	}
)

var (
	localDevice IDevice
)

func Device() IDevice {
	if localDevice == nil {
		panic("implement not found for interface IDevice, forgot register?")
	}
	return localDevice
}

func RegisterDevice(i IDevice) {
	localDevice = i
}
