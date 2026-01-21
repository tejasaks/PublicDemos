#!/bin/bash

# Cleanup Script - Removes test containers, images, and volumes
# Usage: ./cleanup.sh [--all]
# 
# By default, cleans up test resources (sql-ai-test-* containers/images)
# Use --all flag to also clean up default sqlserver-ollama container/image

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

CLEANUP_ALL=false

# Parse arguments
if [ "$1" = "--all" ]; then
    CLEANUP_ALL=true
fi

echo -e "${BLUE}Starting cleanup of test resources...${NC}"
if [ "$CLEANUP_ALL" = true ]; then
    echo -e "${YELLOW}Mode: Cleaning up ALL SQL AI containers and images (including defaults)${NC}"
else
    echo -e "${YELLOW}Mode: Cleaning up test resources only (use --all to include defaults)${NC}"
fi
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

# Test containers (always cleanup)
CONTAINERS=$(docker ps -a --format '{{.Names}}' | grep "^sql-ai-test-" || true)

if [ -n "$CONTAINERS" ]; then
    echo "$CONTAINERS" | while read -r container; do
        remove_container "$container"
    done
else
    echo -e "${BLUE}No test containers found (sql-ai-test-*)${NC}"
fi

# Default sqlserver-ollama container (only if --all flag)
if [ "$CLEANUP_ALL" = true ]; then
    if docker ps -a --format '{{.Names}}' | grep -q "^sqlserver-ollama$"; then
        remove_container "sqlserver-ollama"
    else
        echo -e "${BLUE}No default container found (sqlserver-ollama)${NC}"
    fi
fi
echo ""

# Cleanup images
echo -e "${BLUE}[2/3] Cleaning up images...${NC}"

# Test images (always cleanup)
IMAGES=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep "^sql-ai-custom" || true)

if [ -n "$IMAGES" ]; then
    echo "$IMAGES" | while read -r image; do
        remove_image "$image"
    done
else
    echo -e "${BLUE}No test images found (sql-ai-custom*)${NC}"
fi

# Default sqlserver-ollama image (only if --all flag)
if [ "$CLEANUP_ALL" = true ]; then
    DEFAULT_IMAGES=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep "^sqlserver-ollama" || true)
    if [ -n "$DEFAULT_IMAGES" ]; then
        echo "$DEFAULT_IMAGES" | while read -r image; do
            remove_image "$image"
        done
    else
        echo -e "${BLUE}No default images found (sqlserver-ollama*)${NC}"
    fi
fi
echo ""

# Cleanup volumes
echo -e "${BLUE}[3/3] Cleaning up volumes...${NC}"
remove_volumes "sqlserver_data"
remove_volumes "sqldata"
remove_volumes "caddy_data"
remove_volumes "ollama_data"
remove_volumes "ollama-models"
remove_volumes "minio_data"
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
