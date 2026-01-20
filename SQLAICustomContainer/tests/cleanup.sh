#!/bin/bash

# Cleanup Script - Removes test containers, images, and volumes
# Usage: ./cleanup.sh [container_name] [image_name]
# If no arguments provided, cleans up all sql-ai-test-* resources

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

CONTAINER_PREFIX="${1:-sql-ai-test-}"
IMAGE_PREFIX="${2:-sql-ai-custom}"

echo -e "${BLUE}Starting cleanup of test resources...${NC}"
echo ""

# Function to safely remove container
remove_container() {
    local container_name="$1"
    
    if docker ps -a --format '{{.Names}}' | grep -q "^${container_name}$"; then
        echo -e "${YELLOW}Removing container: $container_name${NC}"
        
        # Stop if running
        if docker ps --format '{{.Names}}' | grep -q "^${container_name}$"; then
            docker stop "$container_name" &> /dev/null || true
        fi
        
        # Remove container
        docker rm -f "$container_name" &> /dev/null || true
        echo -e "${GREEN}✅ Container removed: $container_name${NC}"
    fi
}

# Function to safely remove image
remove_image() {
    local image_name="$1"
    
    if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^${image_name}$"; then
        echo -e "${YELLOW}Removing image: $image_name${NC}"
        docker rmi -f "$image_name" &> /dev/null || true
        echo -e "${GREEN}✅ Image removed: $image_name${NC}"
    fi
}

# Function to remove volumes
remove_volumes() {
    local volume_prefix="$1"
    
    echo -e "${YELLOW}Removing volumes matching: ${volume_prefix}*${NC}"
    
    # List and remove matching volumes
    VOLUMES=$(docker volume ls -q | grep "^${volume_prefix}" || true)
    
    if [ -n "$VOLUMES" ]; then
        echo "$VOLUMES" | while read -r volume; do
            docker volume rm "$volume" &> /dev/null || true
            echo -e "${GREEN}✅ Volume removed: $volume${NC}"
        done
    else
        echo -e "${BLUE}No volumes found matching: ${volume_prefix}*${NC}"
    fi
}

# Cleanup containers
echo -e "${BLUE}[1/3] Cleaning up containers...${NC}"
CONTAINERS=$(docker ps -a --format '{{.Names}}' | grep "^${CONTAINER_PREFIX}" || true)

if [ -n "$CONTAINERS" ]; then
    echo "$CONTAINERS" | while read -r container; do
        remove_container "$container"
    done
else
    echo -e "${BLUE}No containers found matching: ${CONTAINER_PREFIX}*${NC}"
fi
echo ""

# Cleanup images
echo -e "${BLUE}[2/3] Cleaning up images...${NC}"
IMAGES=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep "^${IMAGE_PREFIX}" || true)

if [ -n "$IMAGES" ]; then
    echo "$IMAGES" | while read -r image; do
        remove_image "$image"
    done
else
    echo -e "${BLUE}No images found matching: ${IMAGE_PREFIX}*${NC}"
fi
echo ""

# Cleanup volumes
echo -e "${BLUE}[3/3] Cleaning up volumes...${NC}"
remove_volumes "sqldata"
remove_volumes "ollama-models"
remove_volumes "minio-data"
echo ""

# Optional: Prune unused resources
echo -e "${YELLOW}Do you want to prune unused Docker resources (dangling images, stopped containers)? [y/N]${NC}"
read -r PRUNE_RESPONSE

if [[ "$PRUNE_RESPONSE" =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Pruning unused Docker resources...${NC}"
    docker system prune -f
    echo -e "${GREEN}✅ Pruning complete${NC}"
fi

echo ""
echo -e "${GREEN}✅ Cleanup complete!${NC}"
