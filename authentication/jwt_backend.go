package authentication

type JWTAuthenticationBackend struct {
	privateKey  *rsa.PrivateKey
	PublicKey   *rsa.PublicKey
	AuthBackend backends.AuthBackend
}

const (
	tokenDuration = 72
	expireOffset  = 3600
)

var authBackendInstance *JWTAuthenticationBackend = nil

// Token - container for jwt.Token for encoding
type Token struct {
	Token *jwt.Token
}

func (t *Token) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(t)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeToken(data []byte) (*Token, error) {
	var t *Token
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func InitJWTAuthenticationBackend(ab backends.AuthBackend) *JWTAuthenticationBackend {
	if authBackendInstance == nil {
		authBackendInstance = &JWTAuthenticationBackend{
			privateKey:  getPrivateKey(),
			PublicKey:   getPublicKey(),
			AuthBackend: ab,
		}
	}

	return authBackendInstance
}

func (backend *JWTAuthenticationBackend) GenerateToken(userUUID string) (string, error) {
	token := jwt.New(jwt.SigningMethodRS512)
	token.Claims["exp"] = time.Now().Add(time.Hour * time.Duration(Get().JWTExpirationDelta)).Unix()
	token.Claims["iat"] = time.Now().Unix()
	token.Claims["sub"] = userUUID
	tokenString, err := token.SignedString(backend.privateKey)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("got error while generating JWT token")
		return "", err
	}
	return tokenString, nil
}

