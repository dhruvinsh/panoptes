# PlatformIO post-build step: merge bootloader, partition table, boot_app0 and
# the app into one image that flashes at offset 0 (used for release assets):
#   esptool --chip esp32 write-flash 0 panoptes-factory.bin
Import("env")  # noqa: F821  (provided by PlatformIO)


def merge_factory(source, target, env):
    build = env.subst("$BUILD_DIR")
    out = f"{build}/factory.bin"
    images = [(off, env.subst(img)) for off, img in env.get("FLASH_EXTRA_IMAGES", [])]
    images.append((env.subst("$ESP32_APP_OFFSET"), f"{build}/firmware.bin"))
    args = " ".join(f"{off} {img}" for off, img in images)
    env.Execute(f'"$UPLOADER" --chip esp32 merge-bin --flash-mode keep --flash-size 4MB -o "{out}" {args}')


env.AddPostAction("$BUILD_DIR/${PROGNAME}.bin", merge_factory)  # noqa: F821
