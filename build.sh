#!/bin/bash
set -xeuo pipefail

VERSION="0.5.0"
IMAGE="docker.io/maintainer64/srl-labs-clabernetes"
BUILDER="${1:-docker}"
ARGS="--build-arg VERSION=$VERSION --no-cache --progress=plain"
if [[ $BUILDER == "podman" ]]; then
  ARGS+=" --format docker"
fi
echo $ARGS

# Функция для обработки ошибок
error_handler() {
    echo "Ошибка в строке $1, команда: $2"
    exit 1
}

trap 'error_handler $LINENO "$BASH_COMMAND"' ERR

# Функция для сборки и пуша образов
build_and_push() {
    local image_name=$1
    local dockerfile=$2
    local tags=("$image_name:$VERSION" "$image_name:latest")

    echo "🔨 Сборка образа: $image_name"
    $BUILDER build -t "${tags[0]}" -t "${tags[1]}" -f "$dockerfile" . $ARGS

    for tag in "${tags[@]}"; do
        echo "📤 Пуш образа: $tag"
        $BUILDER push "$tag"
    done
}

build_with_pull() {
    # Заменяем файл storage.conf в оригинальном контейнере и скачиваем образы
    local image_name=$1
    local dockerfile=$2
    local base_image=$3
    local tags=("$image_name:$VERSION" "$image_name:latest")
    local inject_script
    inject_script=$(cat "build/podman-images/inject.sh")

    echo "🐋 Запуск контейнера для выполнения podman pull"
    container_id=$($BUILDER run -d --entrypoint "sleep" --privileged "$base_image" infinity)

    echo "📥 Выполнение podman pull внутри контейнера"
    $BUILDER exec "$container_id" sh -c "$inject_script"

    echo "💾 Коммит изменений в новый образ"
    $BUILDER commit "$container_id" "${tags[0]}"
    $BUILDER tag "${tags[0]}" "${tags[1]}"

    echo "🧹 Остановка и удаление контейнера"
    $BUILDER stop "$container_id"
    $BUILDER rm "$container_id"

    for tag in "${tags[@]}"; do
        echo "📤 Пуш образа: $tag"
        $BUILDER push "$tag"
    done
}

# Build launcher
echo "🚀 Сборка launcher образа"
build_and_push "$IMAGE-launcher" "./build/launcher.Dockerfile"

# Build launcher with images
echo "🚀 Загрузка образов в launcher"
build_with_pull "$IMAGE-launcher-images" "./build/launcher-images.Dockerfile" "$IMAGE-launcher"

echo "✅ Все образы успешно собраны и запушены!"