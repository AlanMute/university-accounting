package university_accounting

import (
	"context"
	"database/sql"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

var (
	redisClient *redis.Client
	mongoClient *mongo.Client
	neoClient   neo4j.Driver
	pgdbClient  *sql.DB
	esClient    *elasticsearch.Client
	ctx         = context.Background()
)

func main() {
	setupDbs()
	closeAll()
}

func setupDbs() {
	var err error

	// Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	logrus.Info("Connected to Redis!")

	// MongoDB
	mongoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	mongoClient, err = mongo.Connect(mongoCtx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		logrus.Fatalf("Failed to create MongoDB client: %v", err)
	}
	if err = mongoClient.Ping(mongoCtx, nil); err != nil {
		logrus.Fatalf("Failed to ping MongoDB: %v", err)
	}
	logrus.Info("Connected to MongoDB!")

	// Neo4j
	neoClient, err = neo4j.NewDriver("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "password123", ""))
	if err != nil {
		logrus.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	if err = neoClient.VerifyConnectivity(); err != nil {
		logrus.Fatalf("Failed to verify Neo4j connection: %v", err)
	}
	logrus.Info("Connected to Neo4j!")

	// PostgreSQL
	pgdbClient, err = sql.Open("postgres", "user=postgres password=password dbname=yourdb sslmode=disable")
	if err != nil {
		logrus.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	if err = pgdbClient.Ping(); err != nil {
		logrus.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	logrus.Info("Connected to PostgreSQL!")

	// ElasticSearch
	esClient, err = elasticsearch.NewDefaultClient()
	if err != nil {
		logrus.Fatalf("Failed to connect to ElasticSearch: %v", err)
	}
	res, err := esClient.Info()
	if err != nil {
		logrus.Fatalf("Failed to get ElasticSearch info: %v", err)
	}
	defer res.Body.Close()
	logrus.Info("Connected to ElasticSearch!")
}

func closeAll() {
	_ = redisClient.Close()
	_ = mongoClient.Disconnect(ctx)
	_ = neoClient.Close()
	_ = pgdbClient.Close()
	logrus.Info("All connections closed!")
}
