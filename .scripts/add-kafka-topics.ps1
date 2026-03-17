
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic user.created --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic user.updated --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic user.deleted --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic route.created --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic event.completed --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic media.uploaded --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic user.events.retry --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic user.events.dlq --partitions 1 --replication-factor 1
docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic user.achievement_earned --partitions 1 --replication-factor 1

echo "=== Available topics ==="

docker exec trip-builder-backend-kafka kafka-topics --bootstrap-server localhost:9092 --list
