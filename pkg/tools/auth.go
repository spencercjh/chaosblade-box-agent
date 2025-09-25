/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tools

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	AccessKeyName = "AK"
	SecretKeyName = "SK"
	Delimiter     = "="

	AppInstanceKeyName = "appInstance"
	AppGroupKeyName    = "appGroup"
)

var (
	AppFile        = path.Join(GetCurrentDirectory(), ".chaos.app")
	localAccessKey = ""
	localSecureKey = ""
	mutex          = sync.RWMutex{}
)

// GetAccessKey
func GetAccessKey() string {
	mutex.RLock()
	defer mutex.RUnlock()
	return localAccessKey
}

// GetSecureKey
func GetSecureKey() string {
	mutex.RLock()
	defer mutex.RUnlock()
	return localSecureKey
}

// Sign
func Sign(signData string) string {
	sum256 := sha256.Sum256([]byte((signData + localSecureKey)))
	encodeToString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%x", string(sum256[:]))))
	return encodeToString
}

func splitToChunk(encodeString string, size int) string {
	if len(encodeString) < size {
		return encodeString
	}
	temp := make([]string, 0, len(encodeString)/size+1)
	for len(encodeString) > 0 {
		if len(encodeString) < size {
			size = len(encodeString)
		}
		temp, encodeString = append(temp, encodeString[:size]), encodeString[size:]
	}
	return strings.Join(temp, "")
}

func Auth(sign, signData string) bool {
	expectSign := Sign(signData)
	if expectSign != sign {
		log.Warningf("Sign not equal. ak: %s, expectSign: %s, receiveSign: %s", GetAccessKey(), expectSign, sign)
		return false
	}
	return true
}

// Record AK/SK to file
func RecordSecretKeyToFile(accessKey, secretKey string) error {
	if accessKey == "" || secretKey == "" {
		log.Warningln("key: ", accessKey, secretKey)
		return errors.New("accessKey or secretKey is empty")
	}

	keys := map[string]string{
		AccessKeyName: accessKey,
		SecretKeyName: secretKey,
	}
	err := RecordMapToFile(keys, path.Join(GetUserHome(), ".chaos.cert"), true)
	if err != nil {
		return err
	}
	localAccessKey = accessKey
	localSecureKey = secretKey
	return nil
}

// RecordApplicationToFile
func RecordApplicationToFile(appInstance, appGroup string, truncate bool) error {
	keys := map[string]string{
		AppInstanceKeyName: appInstance,
		AppGroupKeyName:    appGroup,
	}
	return RecordMapToFile(keys, AppFile, truncate)
}

func RecordMapToFile(data map[string]string, filePath string, truncate bool) error {
	if len(data) == 0 {
		return nil
	}
	mutex.Lock()
	defer mutex.Unlock()
	flag := os.O_WRONLY | os.O_CREATE
	if truncate {
		flag = flag | os.O_TRUNC
	}
	file, err := os.OpenFile(filePath, flag, 0o666)
	defer file.Close()
	if err != nil {
		log.WithField("file", filePath).WithError(err).Errorf("record data to file failed")
		return err
	}
	for key, value := range data {
		_, err := file.WriteString(strings.Join([]string{key, value}, Delimiter) + "\n")
		if err != nil {
			log.WithFields(log.Fields{
				"file":  filePath,
				"key":   key,
				"value": value,
			}).WithError(err).Errorf("write data to file failed")
			return err
		}
	}
	return nil
}

// ReadAppInfoFromFile returns the local application record
func ReadAppInfoFromFile() (appInstance, appGroup string, err error) {
	bytes, err := ioutil.ReadFile(AppFile)
	if err != nil {
		return "", "", err
	}
	content := strings.TrimSpace(string(bytes))
	slice := strings.Split(content, "\n")
	if len(slice) == 0 {
		return "", "", fmt.Errorf("empty content")
	}
	for _, value := range slice {
		kv := strings.SplitN(value, Delimiter, 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case AppInstanceKeyName:
			appInstance = kv[1]
		case AppGroupKeyName:
			appGroup = kv[1]
		}
	}
	return appInstance, appGroup, err
}
