package utils

import (
	"VentureBackend/static/models"
	"VentureBackend/static/profiles"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var (
	MongoClient       *mongo.Client
	UserCollection    *mongo.Collection
	ProfileCollection *mongo.Collection
	MobileCollection  *mongo.Collection
	FriendsCollection *mongo.Collection
)

func InitMongoDB() error {
	dbName := os.Getenv("DB_NAME")
	uri := os.Getenv("MONGO_URI") + "/" + os.Getenv("DB_NAME")
	if uri == "" {
		MongoDB.Log("MONGO_URI not set in environment variables")
		return ErrMissingEnv("MONGODB_URI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	MongoDB.Logf("Connecting to MongoDB at %s...", uri)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		MongoDB.Log("Failed to connect to MongoDB:", err)
		return err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		MongoDB.Log("MongoDB ping failed:", err)
		return err
	}

	MongoDB.Log("MongoDB connection established successfully")
	MongoClient = client
	db := client.Database(dbName)
	UserCollection = db.Collection("users")
	ProfileCollection = db.Collection("profiles")
	MobileCollection = db.Collection("mobile")
	FriendsCollection = db.Collection("friends")
	return nil
}

type ErrMissingEnv string

func (e ErrMissingEnv) Error() string {
	return "environment variable missing: " + string(e)
}

type BasicAuth struct {
	Username string
	Password string
}

func GenerateRandomID() string {
	return uuid.NewString()
}

func DecodeBasicAuth(header string) (BasicAuth, error) {
	const prefix = "Basic "

	if len(header) < len(prefix) || strings.ToLower(header[:len(prefix)]) != strings.ToLower(prefix) {
		return BasicAuth{}, errors.New("invalid authorization header prefix")
	}

	encoded := strings.TrimSpace(header[len(prefix):])
	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return BasicAuth{}, err
	}

	parts := strings.SplitN(string(decodedBytes), ":", 2)
	if len(parts) != 2 {
		return BasicAuth{}, errors.New("invalid basic auth format")
	}

	return BasicAuth{Username: parts[0], Password: parts[1]}, nil
}

func VerifyPassword(input, stored string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(input))
	return err == nil
}

func GenerateClientToken(clientID string, duration time.Duration) (string, time.Duration) {
	claims := jwt.RegisteredClaims{
		Subject:   clientID,
		Issuer:    "fortnite",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
		ID:        GenerateRandomID(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	JWTSecret := os.Getenv("JWT_SECRET")
	signed, _ := token.SignedString([]byte(JWTSecret))
	return signed, duration
}

func GenerateUserAccessToken(accountID, clientID string, duration time.Duration) (string, time.Duration) {
	claims := jwt.RegisteredClaims{
		Audience:  []string{accountID},
		Subject:   clientID,
		Issuer:    accountID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
		ID:        GenerateRandomID(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	JWTSecret := os.Getenv("JWT_SECRET")
	signed, _ := token.SignedString([]byte(JWTSecret))
	return signed, duration
}

func GenerateUserRefreshToken(accountID, clientID string, duration time.Duration) (string, time.Duration) {
	claims := jwt.RegisteredClaims{
		Audience:  []string{accountID},
		Subject:   clientID,
		Issuer:    accountID + "_refresh",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
		ID:        GenerateRandomID(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	JWTSecret := os.Getenv("JWT_SECRET")
	signed, _ := token.SignedString([]byte(JWTSecret))
	return signed, duration
}

func GenerateDeviceID() string {
	return GenerateRandomID()
}

type VersionInfo struct {
	Season int
	Build  float64
	CL     string
	Lobby  string
}

func GetVersionInfo(r *http.Request) VersionInfo {
	memory := VersionInfo{
		Season: 0,
		Build:  0.0,
		CL:     "0",
		Lobby:  "",
	}
	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		memory.Lobby = "LobbySeason0"
		return memory
	}
	var cl string
	parts := strings.Split(userAgent, "-")
	if len(parts) >= 4 {
		subParts := strings.Split(parts[3], ",")
		if clCandidate := subParts[0]; isNumeric(clCandidate) {
			cl = clCandidate
		} else {
			subParts = strings.Split(parts[3], " ")
			if clCandidate := subParts[0]; isNumeric(clCandidate) {
				cl = clCandidate
			}
		}
	}
	if cl == "" && len(parts) >= 2 {
		subParts := strings.Split(parts[1], "+")
		if clCandidate := subParts[0]; isNumeric(clCandidate) {
			cl = clCandidate
		}
	}
	if strings.Contains(userAgent, "Release-") {
		buildRaw := strings.Split(userAgent, "Release-")[1]
		buildPart := strings.Split(buildRaw, "-")[0]
		if strings.Count(buildPart, ".") == 2 {
			vals := strings.Split(buildPart, ".")
			buildPart = vals[0] + "." + vals[1] + vals[2]
		}
		buildFloat, err := strconv.ParseFloat(buildPart, 64)
		if err == nil {
			seasonInt, _ := strconv.Atoi(strings.Split(buildPart, ".")[0])
			memory.Season = seasonInt
			memory.Build = buildFloat
			memory.CL = cl
			memory.Lobby = "LobbySeason" + strconv.Itoa(seasonInt)
			return memory
		}
	}
	clInt, _ := strconv.Atoi(cl)
	switch {
	case clInt < 3724489:
		memory.Season = 0
		memory.Build = 0.0
		memory.CL = cl
		memory.Lobby = "LobbySeason0"
	case clInt <= 3790078:
		memory.Season = 1
		memory.Build = 1.0
		memory.CL = cl
		memory.Lobby = "LobbySeason1"
	default:
		memory.Season = 2
		memory.Build = 2.0
		memory.CL = cl
		memory.Lobby = "LobbyWinterDecor"
	}
	return memory
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	r := time.Now().UnixNano()
	for i := 0; i < length; i++ {
		index := r % int64(len(charset))
		sb.WriteByte(charset[index])
		r = r / 7
	}
	return sb.String()
}

func RegisterUser(discordId *string, username, email, plainPassword string, isServer bool) bool {
	email = strings.ToLower(email)
	if username == "" || email == "" || plainPassword == "" {
		Error.Log("RegisterUser: Missing required fields")
		return false
	}

	if discordId != nil {
		var existingUser bson.M
		err := UserCollection.FindOne(context.TODO(), bson.M{"discordId": *discordId}).Decode(&existingUser)
		if err == nil {
			Error.Log("RegisterUser: Discord ID already exists")
			return false
		}
	}

	var existingEmail bson.M
	err := UserCollection.FindOne(context.TODO(), bson.M{"email": email}).Decode(&existingEmail)
	if err == nil {
		Error.Log("RegisterUser: Email already exists")
		return false
	}

	emailRegex := regexp.MustCompile(`^([a-zA-Z0-9_\.\-])+\@(([a-zA-Z0-9\-])+\.)+([a-zA-Z0-9]{2,4})+$`)
	if !emailRegex.MatchString(email) {
		Error.Log("RegisterUser: Invalid email format")
		return false
	}

	if !isServer {
		if len(username) >= 25 || len(username) < 3 || len(plainPassword) >= 128 || len(plainPassword) < 4 {
			Error.Log("RegisterUser: Username or password length invalid")
			return false
		}
	}

	allowedChars := " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	for _, char := range username {
		if !strings.ContainsRune(allowedChars, char) {
			Error.Log("RegisterUser: Username contains invalid characters")
			return false
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		Error.Log("RegisterUser: Password hashing failed:", err)
		return false
	}

	accountId := strings.ReplaceAll(GenerateRandomID(), "-", "")
	matchmakingId := strings.ReplaceAll(GenerateRandomID(), "-", "")
	createdTime := time.Now().UTC().Format(time.RFC3339)

	userDoc := bson.M{
		"created":       createdTime,
		"banned":        false,
		"discordId":     discordId,
		"accountId":     accountId,
		"username":      username,
		"email":         email,
		"password":      string(hashedPassword),
		"matchmakingId": matchmakingId,
		"isServer":      isServer,
		"acceptedEULA":  false,
	}
	_, err = UserCollection.InsertOne(context.TODO(), userDoc)
	if err != nil {
		Error.Log("RegisterUser: Failed to insert user document:", err)
		return false
	}

	mobileDoc := bson.M{
		"accountId": accountId,
		"email":     email,
		"password":  "rh+" + generateRandomPassword(6),
	}
	_, err = MobileCollection.InsertOne(context.TODO(), mobileDoc)
	if err != nil {
		Error.Log("RegisterUser: Failed to insert mobile document:", err)
		return false
	}

	profilesData, err := profiles.CreateProfiles(accountId)
	if err != nil {
		Error.Log("RegisterUser: Failed to create profiles data:", err)
		return false
	}

	profileDoc := bson.M{
		"accountId": accountId,
		"profiles":  profilesData,
	}
	_, err = ProfileCollection.InsertOne(context.TODO(), profileDoc)
	if err != nil {
		Error.Log("RegisterUser: Failed to insert profile document:", err)
		return false
	}

	emptyFriendList := models.FriendList{
		Accepted: []models.FriendEntry{},
		Incoming: []models.FriendEntry{},
		Outgoing: []models.FriendEntry{},
		Blocked:  []models.FriendEntry{},
	}
	friendDoc := bson.M{
		"accountId": accountId,
		"list":      emptyFriendList,
	}
	_, err = FriendsCollection.InsertOne(context.TODO(), friendDoc)
	if err != nil {
		Error.Log("RegisterUser: Failed to insert friend document:", err)
		return false
	}

	return true
}

func DeleteUser(accountId string) bool {
	if accountId == "" {
		Error.Log("DeleteUser: accountId is required")
		return false
	}

	_, err := UserCollection.DeleteOne(context.TODO(), bson.M{"accountId": accountId})
	if err != nil {
		Error.Log("DeleteUser: Failed to delete user document:", err)
		return false
	}

	_, err = ProfileCollection.DeleteOne(context.TODO(), bson.M{"accountId": accountId})
	if err != nil {
		Error.Log("DeleteUser: Failed to delete profile document:", err)
		return false
	}

	_, err = MobileCollection.DeleteOne(context.TODO(), bson.M{"accountId": accountId})
	if err != nil {
		Error.Log("DeleteUser: Failed to delete mobile document:", err)
		return false
	}

	_, err = FriendsCollection.DeleteOne(context.TODO(), bson.M{"accountId": accountId})
	if err != nil {
		Error.Log("DeleteUser: Failed to delete friend document:", err)
		return false
	}

	_, err = FriendsCollection.UpdateMany(
		context.TODO(),
		bson.M{},
		bson.M{
			"$pull": bson.M{
				"list.Accepted": accountId,
				"list.Incoming": accountId,
				"list.Outgoing": accountId,
				"list.Blocked":  accountId,
			},
		},
	)
	if err != nil {
		Error.Log("DeleteUser: Failed to clean up references in friends lists:", err)
	}

	return true
}

func FindUserByDiscordId(id string) (*models.User, error) {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := collection.FindOne(ctx, bson.M{"discordId": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func FindUserByEmail(email string) (*models.User, error) {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func FindMobileUserByEmail(email string) (*models.Mobile, error) {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("mobile")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.Mobile
	err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func FindUserByAccountID(accountID string) (*models.User, error) {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("users")
	var user models.User
	err := collection.FindOne(context.TODO(), bson.M{"accountId": accountID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func FindUserByMatchmakingID(matchmakingID string) (*models.User, error) {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("users")
	var user models.User
	err := collection.FindOne(context.TODO(), bson.M{"matchmakingId": matchmakingID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func FindUserByUsername(username string) (*models.User, error) {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("users")
	var user models.User
	err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func FindProfileByAccountID(accountID string) (*models.Profiles, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var profile models.Profiles
	err := ProfileCollection.FindOne(ctx, bson.M{"accountId": accountID}).Decode(&profile)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func FindFriendByAccountID(accountID string) (*models.Friends, error) {
	dbName := os.Getenv("DB_NAME")
	collection := MongoClient.Database(dbName).Collection("friends")

	var friend models.Friends
	err := collection.FindOne(context.TODO(), bson.M{"accountId": accountID}).Decode(&friend)
	if err != nil {
		return nil, err
	}
	return &friend, nil
}

func FindMobileByAccountID(accountID string) (*models.Mobile, error) {
	dbName := os.Getenv("DB_NAME")
	collection := MongoClient.Database(dbName).Collection("mobile")

	var mobile models.Mobile
	err := collection.FindOne(context.TODO(), bson.M{"accountId": accountID}).Decode(&mobile)
	if err != nil {
		return nil, err
	}
	return &mobile, nil
}

func LoadJSON(path string) (interface{}, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func UpdateFriendList(friend *models.Friends) error {
	db := os.Getenv("DB_NAME")
	collection := MongoClient.Database(db).Collection("friends")
	filter := bson.M{"accountId": friend.AccountID}
	update := bson.M{"$set": bson.M{"list": friend.List}}
	_, err := collection.UpdateOne(context.TODO(), filter, update)
	return err
}
