package device

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/crypto/gaes"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gbase64"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/google/uuid"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
)

func init() {
	service.RegisterDevice(&sDevice{})
}

type (
	sDevice struct {
	}
)

func (d *sDevice) CreateDevice(ctx context.Context, deviceKey string, dev *entity.Device) (id string, err error) {
	exist, err := dao.Device.Ctx(ctx).Exist(dao.Device.Columns().Name, dev.Name)
	if err != nil {
		return
	}
	if exist {
		err = response.CodeConflict.WithDetail("name already exists")
		return
	}

	id = uuid.NewString()
	dev.Id = id
	dev.Enabled = 1

	// generate rsa key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	// encode private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	encryptedPrivateKey, err := gaes.Encrypt(privateKeyPEM, d.buildEncryptKey(deviceKey))
	if err != nil {
		err = response.CodeInvalidParameter.WithDetail("failed to encrypt private key")
		return
	}
	dev.PrivateKey = gbase64.EncodeToString(encryptedPrivateKey)

	// encode public key to PEM format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", err
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	dev.PublicKey = string(publicKeyPEM)
	dev.CreatedAt = gtime.Now()

	_, err = dao.Device.Ctx(ctx).Insert(dev)
	return
}

func (d *sDevice) GetDeviceInfo(ctx context.Context, deviceId string) (dev *model.Device, err error) {
	device, err := d.GetDeviceById(ctx, deviceId)
	if err != nil {
		return
	}
	dev = &model.Device{
		Id:        device.Id,
		Name:      device.Name,
		CreatedAt: device.CreatedAt,
	}
	results, err := dao.NetworkDevice.Ctx(ctx).Where(g.Map{
		dao.NetworkDevice.Columns().DeviceId: deviceId,
	}).All()
	if err != nil {
		return
	}
	for _, result := range results {
		sub := new(entity.NetworkDevice)
		if err = result.Struct(&sub); err != nil {
			return
		}
		dev.Sub = append(dev.Sub, sub)
	}
	return
}

func (d *sDevice) GetDeviceById(ctx context.Context, id string) (dev *entity.Device, err error) {
	record, err := dao.Device.Ctx(ctx).Where(dao.Device.Columns().Id, id).One()
	if err != nil {
		return
	}
	dev, err = d.parseDevice(record)
	return
}

func (d *sDevice) GetDeviceByName(ctx context.Context, name string) (dev *entity.Device, err error) {
	record, err := dao.Device.Ctx(ctx).Where(dao.Device.Columns().Name, name).One()
	if err != nil {
		return
	}
	dev, err = d.parseDevice(record)
	return
}

func (d *sDevice) parseDevice(record gdb.Record) (dev *entity.Device, err error) {
	dev = new(entity.Device)
	err = record.Struct(&dev)
	if errors.Is(gerror.Cause(err), sql.ErrNoRows) {
		err = response.CodeNotFound
	}
	return
}

func (d *sDevice) VerifyByName(ctx context.Context, name, key, nonce, signature string) (dev *entity.Device, err error) {
	dev, err = d.GetDeviceByName(ctx, name)
	if err != nil {
		return
	}
	err = d.verify(ctx, dev, key, nonce, signature)
	return
}

func (d *sDevice) VerifyById(ctx context.Context, id, key, nonce, signature string) (dev *entity.Device, err error) {
	dev, err = d.GetDeviceById(ctx, id)
	if err != nil {
		return
	}
	err = d.verify(ctx, dev, key, nonce, signature)
	return
}

func (d *sDevice) verify(_ context.Context, dev *entity.Device, key string, nonce, signature string) (err error) {
	if dev.Enabled != 1 {
		err = response.CodeInvalidParameter.WithDetail("device disabled")
		return
	}
	encrypted, err := gbase64.DecodeString(dev.PrivateKey)
	if err != nil {
		err = response.CodeDefaultFailure.WithDetail(fmt.Sprintf("broken keypair: %s", err.Error()))
		return
	}
	decryptedKey, err := gaes.Decrypt(encrypted, d.buildEncryptKey(key))
	if err != nil {
		err = response.CodePermissionDeny.WithDetail("invalid key")
		return
	}
	// convert byte to *rsa.PrivateKey
	block, _ := pem.Decode(decryptedKey)
	if block == nil {
		err = response.CodeDefaultFailure.WithDetail("failed to decode PEM block")
		return
	}
	pk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		err = response.CodeDefaultFailure.WithDetail("failed to parse private key")
		return
	}
	// decode signature -> nonce
	bs, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, pk, []byte(signature), []byte("device"))
	if err != nil {
		err = response.CodePermissionDeny.WithDetail("invalid signature")
		return
	}
	if string(bs) != nonce {
		err = response.CodePermissionDeny.WithDetail("invalid nonce")
	}
	return
}

func (d *sDevice) buildEncryptKey(key string) []byte {
	res := []byte(key)
	if len(res) < 16 {
		res = append(res, make([]byte, 16-len(res))...)
	}
	return res[:16]
}
