package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"to-do-list/internal/models"
)

type MongoTaskRepository struct {
	collection *mongo.Collection
}

func NewMongoTaskRepository(database *mongo.Database, collectionName string) *MongoTaskRepository {
	return &MongoTaskRepository{
		collection: database.Collection(collectionName),
	}
}

func (repository *MongoTaskRepository) Create(repositoryContext context.Context, task *models.Task) error {
	if _, err := repository.collection.InsertOne(repositoryContext, task); err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	return nil
}

func (repository *MongoTaskRepository) List(repositoryContext context.Context, filter models.TaskFilter) ([]models.Task, error) {
	cursor, err := repository.collection.Find(repositoryContext, buildTaskFilter(filter), options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("find tasks: %w", err)
	}

	var tasks []models.Task
	if err := cursor.All(repositoryContext, &tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	return tasks, nil
}

func (repository *MongoTaskRepository) GetByID(repositoryContext context.Context, taskID string) (*models.Task, error) {
	var task models.Task

	if err := repository.collection.FindOne(repositoryContext, bson.M{"_id": taskID}).Decode(&task); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("find task by id: %w", err)
	}

	return &task, nil
}

func (repository *MongoTaskRepository) Update(repositoryContext context.Context, taskID string, taskUpdate models.TaskUpdate) (*models.Task, error) {
	var updatedTask models.Task

	result := repository.collection.FindOneAndUpdate(
		repositoryContext,
		bson.M{
			"_id":    taskID,
			"status": bson.M{"$ne": string(models.TaskStatusCompleted)},
		},
		bson.M{"$set": buildTaskUpdate(taskUpdate)},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	if err := result.Decode(&updatedTask); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("update task: %w", err)
	}

	return &updatedTask, nil
}

func (repository *MongoTaskRepository) Delete(repositoryContext context.Context, taskID string) error {
	deleteResult, err := repository.collection.DeleteOne(repositoryContext, bson.M{"_id": taskID})
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	if deleteResult.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}

func buildTaskFilter(filter models.TaskFilter) bson.M {
	query := bson.M{}

	if filter.Status != nil {
		query["status"] = string(*filter.Status)
	}

	if filter.Priority != nil {
		query["priority"] = string(*filter.Priority)
	}

	return query
}

func buildTaskUpdate(taskUpdate models.TaskUpdate) bson.M {
	update := bson.M{
		"updatedAt": time.Now().UTC(),
	}

	if taskUpdate.Title != nil {
		update["title"] = *taskUpdate.Title
	}

	if taskUpdate.Description != nil {
		update["description"] = *taskUpdate.Description
	}

	if taskUpdate.Status != nil {
		update["status"] = string(*taskUpdate.Status)
	}

	if taskUpdate.Priority != nil {
		update["priority"] = string(*taskUpdate.Priority)
	}

	if taskUpdate.DueDate != nil {
		update["dueDate"] = *taskUpdate.DueDate
	}

	return update
}
