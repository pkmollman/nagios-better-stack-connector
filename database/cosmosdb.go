package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

func CreateEventItem(client *azcosmos.Client, databaseName, containerName, partitionKey string, item EventItem) error {
	// Create container client
	containerClient, err := client.NewContainer(databaseName, containerName)
	if err != nil {
		return fmt.Errorf("failed to create a container client: %s", err)
	}

	// Specifies the value of the partiton key
	pk := azcosmos.NewPartitionKeyString(partitionKey)

	b, err := json.Marshal(item)
	if err != nil {
		fmt.Println("marshal error:", err)
		return err
	}
	// setting item options upon creating ie. consistency level
	itemOptions := azcosmos.ItemOptions{
		ConsistencyLevel: azcosmos.ConsistencyLevelSession.ToPtr(),
	}
	ctx := context.TODO()
	_, err = containerClient.CreateItem(ctx, pk, b, &itemOptions)

	if err != nil {
		return err
	}

	return nil
}

// func addIncidentIdToItem(client *azcosmos.Client, databaseName, containerName, partitionKey string, item any) error {
// 	// Create container client
// 	containerClient, err := client.NewContainer(databaseName, containerName)
// 	if err != nil {
// 		return fmt.Errorf("failed to create a container client: %s", err)
// 	}

// 	// Specifies the value of the partiton key
// 	pk := azcosmos.NewPartitionKeyString(partitionKey)

// 	b, err := json.Marshal(item)
// 	if err != nil {
// 		fmt.Println("marshal error:", err)
// 		return err
// 	}
// 	// setting item options upon creating ie. consistency level
// 	itemOptions := azcosmos.ItemOptions{
// 		ConsistencyLevel: azcosmos.ConsistencyLevelSession.ToPtr(),
// 	}
// 	ctx := context.TODO()
// 	itemResponse, err := containerClient.CreateItem(ctx, pk, b, &itemOptions)

// 	if err != nil {
// 		return err
// 	}
// 	log.Printf("Status %d. Item %v created. ActivityId %s. Consuming %v Request Units.\n", itemResponse.RawResponse.StatusCode, pk, itemResponse.ActivityID, itemResponse.RequestCharge)

// 	return nil
// }

// func GetAllEventItems(client *azcosmos.Client, databaseName, containerName, partitionKey string) ([]EventItem, error) {
// 	pk := azcosmos.NewPartitionKeyString(partitionKey)
// 	// Create container client
// 	containerClient, err := client.NewContainer(databaseName, containerName)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create a container client: %s", err)
// 	}
// 	queryPager := containerClient.NewQueryItemsPager("SELECT * FROM c", pk, nil)
// 	allItems := []EventItem{}
// 	for queryPager.More() {
// 		queryResponse, err := queryPager.NextPage(context.Background())
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get next page: %v", err)
// 		}
// 		for _, respItem := range queryResponse.Items {
// 			var item EventItem
// 			_ = json.Unmarshal(respItem, &item)
// 			allItems = append(allItems, item)
// 		}
// 	}
// 	return allItems, nil
}
