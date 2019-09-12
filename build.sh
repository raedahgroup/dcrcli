function packFyneAssets() {
    echo "packing assets with packr2"
    (cd fyne && packr2)
}

function buildFyne() {
    packFyneAssets
    echo "building with go build"
    (cd ./cmd/godcr-fyne && go build)
    cp cmd/godcr-fyne/godcr-fyne .
}

if [[ "$1" = "fyne" ]]; then
    buildFyne
else
    echo "Usage: ./build.sh {interface} e.g. ./build.sh fyne"
fi
