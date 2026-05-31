cmake_minimum_required(VERSION 3.16)
project(runner-wasmedge C)

# The build script passes WASMEDGE_SRC / WASMEDGE_BUILD; fall back to the
# in-tree cache layout.
if(NOT DEFINED WASMEDGE_BUILD)
    if(DEFINED WASMEDGE_BUILD_DIR)
        set(WASMEDGE_BUILD "${WASMEDGE_BUILD_DIR}")
    else()
        set(WASMEDGE_BUILD "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/wasmedge/build")
    endif()
endif()

add_executable(runner-wasmedge runner.c)
# The generated C API header lives under build/include/api/wasmedge/wasmedge.h.
target_include_directories(runner-wasmedge PRIVATE
    "${WASMEDGE_BUILD}/include/api"
)
target_link_libraries(runner-wasmedge
    "${WASMEDGE_BUILD}/lib/api/libwasmedge.so"
    # rpath so the runner finds libwasmedge.so at runtime without LD_LIBRARY_PATH.
    pthread dl m
)
set_target_properties(runner-wasmedge PROPERTIES
    BUILD_RPATH "${WASMEDGE_BUILD}/lib/api"
    INSTALL_RPATH "${WASMEDGE_BUILD}/lib/api"
)
