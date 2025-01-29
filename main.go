// Package directive and module imports:
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	ycsdk "github.com/yandex-cloud/go-sdk"
)

// Type annotation
type Config struct {
    FolderID  string            `json:"folder_id"`
    Username  string            `json:"username"`
    Resources Resources         `json:"resources"`
    Metadata  map[string]string `json:"metadata"`
    Labels    map[string]string `json:"labels"`
}

type Resources struct {
    Image         Image         `json:"image"`
    Name          string        `json:"name"`
    ResourcesSpec ResourcesSpec `json:"resources_spec"`
    BootDiskSpec  BootDiskSpec  `json:"boot_disk_spec"`
    ZoneID        string        `json:"zone_id"`
    PlatformID    string        `json:"platform_id"`
    SubnetID      string        `json:"subnet_id"`
}

type Image struct {
    Family         string `json:"family"`
    FolderFamilyID string `json:"folder_family_id"`
}

type ResourcesSpec struct {
    Memory int64 `json:"memory"`
    Cores  int64 `json:"cores"`
}

type BootDiskSpec struct {
    AutoDelete bool     `json:"auto_delete"`
    DiskSpec   DiskSpec `json:"disk_spec"`
}

type DiskSpec struct {
    TypeID string `json:"type_id"`
    Size   int64  `json:"size"`
}

type Labels struct {
    GoSDK string `json:"go-sdk"`
}

// Function to read the public part of the SSH key:
func LoadSsh(filePath string) string {
    content, err := os.ReadFile(filePath)
    if err != nil {
        log.Fatal(err)
    }
    return string(content)
}

// Function to read the configuration file:
func LoadConfig(filename string) (*Config, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("error opening file: %w", err)
    }
    defer file.Close()

    var cfg Config
    if err := json.NewDecoder(file).Decode(&cfg); err != nil {
        return nil, fmt.Errorf("error decoding JSON: %w", err)
    }

    return &cfg, nil
}

// Function to create a VM:
func createInstance(ctx context.Context, sdk *ycsdk.SDK, pubSsh string, config Config) (*operation.Operation, error) {
    folderID := config.FolderID
    username := config.Username

    resources := config.Resources
    imageFamily := resources.Image.Family
    imageFolderFamilyID := resources.Image.FolderFamilyID
    name := resources.Name
    memory := resources.ResourcesSpec.Memory
    cores := resources.ResourcesSpec.Cores
    bootDiskAutoDelete := resources.BootDiskSpec.AutoDelete
    bootDiskTypeID := resources.BootDiskSpec.DiskSpec.TypeID
    bootDiskSize := resources.BootDiskSpec.DiskSpec.Size
    zone := resources.ZoneID
    platformID := resources.PlatformID
    subnetID := resources.SubnetID
    labels := config.Labels
    metadata := config.Metadata
    for key, value := range metadata {
        updatedValue := strings.ReplaceAll(value, "USERNAME", username)
        updatedValue = strings.ReplaceAll(updatedValue, "SSH_PUBLIC_KEY", pubSsh)
        metadata[key] = updatedValue
    }


    sourceImageID := sourceImage(ctx, sdk, imageFamily, imageFolderFamilyID)
    request := &compute.CreateInstanceRequest{
        FolderId:   folderID,
        Name:       name,
        ZoneId:     zone,
        PlatformId: platformID,
        ResourcesSpec: &compute.ResourcesSpec{
            Cores:  cores,
            Memory: memory,
        },
        BootDiskSpec: &compute.AttachedDiskSpec{
            AutoDelete: bootDiskAutoDelete,
            Disk: &compute.AttachedDiskSpec_DiskSpec_{
                DiskSpec: &compute.AttachedDiskSpec_DiskSpec{
                    TypeId: bootDiskTypeID,
                    Size:   bootDiskSize,
                    Source: &compute.AttachedDiskSpec_DiskSpec_ImageId{
                        ImageId: sourceImageID,
                    },
                },
            },
        },
        Metadata: metadata,
        Labels:   labels,
        NetworkInterfaceSpecs: []*compute.NetworkInterfaceSpec{
            {
                SubnetId: subnetID,
                PrimaryV4AddressSpec: &compute.PrimaryAddressSpec{
                    OneToOneNatSpec: &compute.OneToOneNatSpec{
                        IpVersion: compute.IpVersion_IPV4,
                    },
                },
            },
        },
    }
    op, err := sdk.Compute().Instance().Create(ctx, request)
    return op, err
}

// Function to get the VM image ID:
func sourceImage(ctx context.Context, sdk *ycsdk.SDK, family, folderId string) string {
    image, err := sdk.Compute().Image().GetLatestByFamily(ctx, &compute.GetImageLatestByFamilyRequest{
        FolderId: folderId,
        Family:   family,
    })
    if err != nil {
        log.Fatal(err)
    }
    return image.Id
}

// Entry point of the program:
func main() {
    token := os.Getenv("IAM_TOKEN")
    sshPath := os.Getenv("SSH_PUBLIC_KEY_PATH")

    ctx := context.Background()

    pubSsh := LoadSsh(sshPath)
    config, err := LoadConfig("config.json")
    if err != nil {
        log.Fatal(err)
    }
    sdk, err := ycsdk.Build(ctx, ycsdk.Config{
        Credentials: ycsdk.NewIAMTokenCredentials(token),
    })
    if err != nil {
        log.Fatal(err)
    }
    op, err := sdk.WrapOperation(createInstance(
        ctx, sdk, pubSsh, *config))
    if err != nil {
        log.Fatal(err)
    }
    opId := op.Id()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Running Yandex.Cloud operation. ID: %s\n", opId)
    err = op.Wait(ctx)
    if err != nil {
        log.Fatal(err)
    }
    resp, err := op.Response()
    if err != nil {
        log.Fatal(err)
    }
    instance := resp.(*compute.Instance)
    fmt.Printf("Instance with id %s was created\n'", instance.Id)
}
