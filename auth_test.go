package gobfd

import (
	"testing"

	internalbfd "github.com/WLmutou/gobfd/internal"
	"github.com/google/gopacket/layers"
)

func TestAuthTypeConstants(t *testing.T) {
	if AuthTypeNone != layers.BFDAuthTypeNone {
		t.Errorf("AuthTypeNone mismatch: expected %v, got %v", layers.BFDAuthTypeNone, AuthTypeNone)
	}
	if AuthTypePassword != layers.BFDAuthTypePassword {
		t.Errorf("AuthTypePassword mismatch: expected %v, got %v", layers.BFDAuthTypePassword, AuthTypePassword)
	}
	if AuthTypeKeyedMD5 != layers.BFDAuthTypeKeyedMD5 {
		t.Errorf("AuthTypeKeyedMD5 mismatch: expected %v, got %v", layers.BFDAuthTypeKeyedMD5, AuthTypeKeyedMD5)
	}
	if AuthTypeKeyedSHA1 != layers.BFDAuthTypeKeyedSHA1 {
		t.Errorf("AuthTypeKeyedSHA1 mismatch: expected %v, got %v", layers.BFDAuthTypeKeyedSHA1, AuthTypeKeyedSHA1)
	}
}

func TestAuthConfig(t *testing.T) {
	authConfig := &AuthConfig{
		AuthType: AuthTypeKeyedMD5,
		KeyID:    1,
		Key:      "testsecretkey",
	}

	if authConfig.AuthType != AuthTypeKeyedMD5 {
		t.Errorf("AuthType mismatch: expected %v, got %v", AuthTypeKeyedMD5, authConfig.AuthType)
	}
	if authConfig.KeyID != 1 {
		t.Errorf("KeyID mismatch: expected 1, got %d", authConfig.KeyID)
	}
	if authConfig.Key != "testsecretkey" {
		t.Errorf("Key mismatch: expected 'testsecretkey', got '%s'", authConfig.Key)
	}
}

func TestBuildAuthHeaderPassword(t *testing.T) {
	key := []byte("mypassword")
	authHeader := internalbfd.BuildAuthHeader(layers.BFDAuthTypePassword, 1, 12345, key)

	if authHeader == nil {
		t.Fatal("BuildAuthHeader returned nil for Password type")
	}
	if authHeader.AuthType != layers.BFDAuthTypePassword {
		t.Errorf("AuthType mismatch: expected %v, got %v", layers.BFDAuthTypePassword, authHeader.AuthType)
	}
	if authHeader.KeyID != layers.BFDAuthKeyID(1) {
		t.Errorf("KeyID mismatch: expected 1, got %d", authHeader.KeyID)
	}
	if authHeader.SequenceNumber != layers.BFDAuthSequenceNumber(12345) {
		t.Errorf("SequenceNumber mismatch: expected 12345, got %d", authHeader.SequenceNumber)
	}
	if string(authHeader.Data) != string(key) {
		t.Errorf("Data mismatch: expected '%s', got '%s'", key, authHeader.Data)
	}
}

func TestBuildAuthHeaderKeyedMD5(t *testing.T) {
	key := []byte("mysecretkey")
	authHeader := internalbfd.BuildAuthHeader(layers.BFDAuthTypeKeyedMD5, 2, 67890, key)

	if authHeader == nil {
		t.Fatal("BuildAuthHeader returned nil for KeyedMD5 type")
	}
	if authHeader.AuthType != layers.BFDAuthTypeKeyedMD5 {
		t.Errorf("AuthType mismatch: expected %v, got %v", layers.BFDAuthTypeKeyedMD5, authHeader.AuthType)
	}
	if authHeader.KeyID != layers.BFDAuthKeyID(2) {
		t.Errorf("KeyID mismatch: expected 2, got %d", authHeader.KeyID)
	}
	if authHeader.SequenceNumber != layers.BFDAuthSequenceNumber(67890) {
		t.Errorf("SequenceNumber mismatch: expected 67890, got %d", authHeader.SequenceNumber)
	}
	if len(authHeader.Data) != 16 {
		t.Errorf("MD5 data length mismatch: expected 16, got %d", len(authHeader.Data))
	}
}

func TestBuildAuthHeaderKeyedSHA1(t *testing.T) {
	key := []byte("mysecretkey")
	authHeader := internalbfd.BuildAuthHeader(layers.BFDAuthTypeKeyedSHA1, 3, 11111, key)

	if authHeader == nil {
		t.Fatal("BuildAuthHeader returned nil for KeyedSHA1 type")
	}
	if authHeader.AuthType != layers.BFDAuthTypeKeyedSHA1 {
		t.Errorf("AuthType mismatch: expected %v, got %v", layers.BFDAuthTypeKeyedSHA1, authHeader.AuthType)
	}
	if authHeader.KeyID != layers.BFDAuthKeyID(3) {
		t.Errorf("KeyID mismatch: expected 3, got %d", authHeader.KeyID)
	}
	if authHeader.SequenceNumber != layers.BFDAuthSequenceNumber(11111) {
		t.Errorf("SequenceNumber mismatch: expected 11111, got %d", authHeader.SequenceNumber)
	}
	if len(authHeader.Data) != 20 {
		t.Errorf("SHA1 data length mismatch: expected 20, got %d", len(authHeader.Data))
	}
}

func TestBuildAuthHeaderNone(t *testing.T) {
	authHeader := internalbfd.BuildAuthHeader(layers.BFDAuthTypeNone, 1, 12345, []byte("key"))

	if authHeader != nil {
		t.Errorf("BuildAuthHeader should return nil for None type, got %v", authHeader)
	}
}

func TestComputeAuthDataMD5(t *testing.T) {
	key := []byte("testkey")
	packet := []byte("testpacketdata")
	seqNum := uint32(12345)

	result := internalbfd.ComputeAuthData(layers.BFDAuthTypeKeyedMD5, key, seqNum, packet)

	if len(result) != 16 {
		t.Errorf("MD5 result length mismatch: expected 16, got %d", len(result))
	}

	result2 := internalbfd.ComputeAuthData(layers.BFDAuthTypeKeyedMD5, key, seqNum, packet)
	if string(result) != string(result2) {
		t.Error("MD5 result should be deterministic for same inputs")
	}
}

func TestComputeAuthDataSHA1(t *testing.T) {
	key := []byte("testkey")
	packet := []byte("testpacketdata")
	seqNum := uint32(12345)

	result := internalbfd.ComputeAuthData(layers.BFDAuthTypeKeyedSHA1, key, seqNum, packet)

	if len(result) != 20 {
		t.Errorf("SHA1 result length mismatch: expected 20, got %d", len(result))
	}

	result2 := internalbfd.ComputeAuthData(layers.BFDAuthTypeKeyedSHA1, key, seqNum, packet)
	if string(result) != string(result2) {
		t.Error("SHA1 result should be deterministic for same inputs")
	}
}

func TestComputeAuthDataPassword(t *testing.T) {
	key := []byte("mypassword")
	packet := []byte("testpacketdata")
	seqNum := uint32(12345)

	result := internalbfd.ComputeAuthData(layers.BFDAuthTypePassword, key, seqNum, packet)

	if string(result) != string(key) {
		t.Errorf("Password auth data mismatch: expected '%s', got '%s'", key, result)
	}
}

func TestValidateAuthNone(t *testing.T) {
	bfd := &layers.BFD{
		AuthPresent: false,
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypeNone, 1, []byte("key"))
	if !result {
		t.Error("ValidateAuth should return true for None auth type with no auth present")
	}
}

func TestValidateAuthNoneWithAuthPresent(t *testing.T) {
	bfd := &layers.BFD{
		AuthPresent: true,
		AuthHeader: &layers.BFDAuthHeader{
			AuthType: layers.BFDAuthTypePassword,
			KeyID:    layers.BFDAuthKeyID(1),
			Data:     []byte("password"),
		},
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypeNone, 1, []byte("key"))
	if result {
		t.Error("ValidateAuth should return false for None auth type when auth is present")
	}
}

func TestValidateAuthTypeMismatch(t *testing.T) {
	bfd := &layers.BFD{
		AuthPresent: true,
		AuthHeader: &layers.BFDAuthHeader{
			AuthType: layers.BFDAuthTypePassword,
			KeyID:    layers.BFDAuthKeyID(1),
			Data:     []byte("password"),
		},
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypeKeyedMD5, 1, []byte("key"))
	if result {
		t.Error("ValidateAuth should return false for auth type mismatch")
	}
}

func TestValidateAuthKeyIDMismatch(t *testing.T) {
	bfd := &layers.BFD{
		AuthPresent: true,
		AuthHeader: &layers.BFDAuthHeader{
			AuthType: layers.BFDAuthTypePassword,
			KeyID:    layers.BFDAuthKeyID(2),
			Data:     []byte("password"),
		},
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypePassword, 1, []byte("key"))
	if result {
		t.Error("ValidateAuth should return false for key ID mismatch")
	}
}

func TestValidateAuthPassword(t *testing.T) {
	password := []byte("mysecretpassword")
	bfd := &layers.BFD{
		AuthPresent: true,
		AuthHeader: &layers.BFDAuthHeader{
			AuthType: layers.BFDAuthTypePassword,
			KeyID:    layers.BFDAuthKeyID(1),
			Data:     password,
		},
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypePassword, 1, password)
	if !result {
		t.Error("ValidateAuth should return true for correct password")
	}

	wrongPassword := []byte("wrongpassword")
	result2 := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypePassword, 1, wrongPassword)
	if result2 {
		t.Error("ValidateAuth should return false for wrong password")
	}
}

func TestValidateAuthMD5DataLength(t *testing.T) {
	bfd := &layers.BFD{
		AuthPresent: true,
		AuthHeader: &layers.BFDAuthHeader{
			AuthType: layers.BFDAuthTypeKeyedMD5,
			KeyID:    layers.BFDAuthKeyID(1),
			Data:     []byte("short"),
		},
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypeKeyedMD5, 1, []byte("key"))
	if result {
		t.Error("ValidateAuth should return false for incorrect MD5 data length")
	}
}

func TestValidateAuthSHA1DataLength(t *testing.T) {
	bfd := &layers.BFD{
		AuthPresent: true,
		AuthHeader: &layers.BFDAuthHeader{
			AuthType: layers.BFDAuthTypeKeyedSHA1,
			KeyID:    layers.BFDAuthKeyID(1),
			Data:     []byte("short"),
		},
	}

	result := internalbfd.ValidateAuth(bfd, layers.BFDAuthTypeKeyedSHA1, 1, []byte("key"))
	if result {
		t.Error("ValidateAuth should return false for incorrect SHA1 data length")
	}
}

func TestNewSessionWithAuth(t *testing.T) {
	callback := func(ip string, preState, curState int) error { return nil }

	sess := internalbfd.NewSessionWithAuth(
		"127.0.0.1",
		"127.0.0.2",
		4,
		false,
		1000,
		1000,
		3,
		false,
		0,
		false,
		nil,
		callback,
		layers.BFDAuthTypeKeyedMD5,
		1,
		"testkey",
	)

	if sess.AuthType != layers.BFDAuthTypeKeyedMD5 {
		t.Errorf("AuthType mismatch: expected %v, got %v", layers.BFDAuthTypeKeyedMD5, sess.AuthType)
	}
	if sess.AuthKeyID != 1 {
		t.Errorf("AuthKeyID mismatch: expected 1, got %d", sess.AuthKeyID)
	}
	if sess.AuthKey != "testkey" {
		t.Errorf("AuthKey mismatch: expected 'testkey', got '%s'", sess.AuthKey)
	}
	if sess.XmitAuthSeq == 0 {
		t.Error("XmitAuthSeq should be initialized to a random value")
	}
}

func TestNewSessionWithAuthNone(t *testing.T) {
	callback := func(ip string, preState, curState int) error { return nil }

	sess := internalbfd.NewSessionWithAuth(
		"127.0.0.1",
		"127.0.0.2",
		4,
		false,
		1000,
		1000,
		3,
		false,
		0,
		false,
		nil,
		callback,
		layers.BFDAuthTypeNone,
		0,
		"",
	)

	if sess.AuthType != layers.BFDAuthTypeNone {
		t.Errorf("AuthType mismatch: expected %v, got %v", layers.BFDAuthTypeNone, sess.AuthType)
	}
	if sess.AuthKey != "" {
		t.Errorf("AuthKey should be empty for None auth type, got '%s'", sess.AuthKey)
	}
}
