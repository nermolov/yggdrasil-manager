cmake_minimum_required(VERSION 3.16)
project(runner-wamr C)

# The build script passes WAMR_SRC / WAMR_BUILD / WAMR_FAST_INTERP; fall back
# to the in-tree cache layout and the older WAMR_SRC_DIR name when invoked
# directly.
if(NOT DEFINED WAMR_SRC)
    if(DEFINED WAMR_SRC_DIR)
        set(WAMR_SRC "${WAMR_SRC_DIR}")
    else()
        set(WAMR_SRC "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/wamr/src")
    endif()
endif()
if(NOT DEFINED WAMR_FAST_INTERP)
    if(DEFINED USE_FAST_INTERP)
        set(WAMR_FAST_INTERP ${USE_FAST_INTERP})
    else()
        set(WAMR_FAST_INTERP 0)
    endif()
endif()

if(WAMR_FAST_INTERP)
    set(_default_build "${WAMR_SRC}/../build-fast")
    set(TARGET_NAME "runner-wamr-fast")
else()
    set(_default_build "${WAMR_SRC}/../build-classic")
    set(TARGET_NAME "runner-wamr-classic")
endif()
if(NOT DEFINED WAMR_BUILD)
    set(WAMR_BUILD "${_default_build}")
endif()

add_executable(${TARGET_NAME} runner.c)
target_compile_definitions(${TARGET_NAME} PRIVATE USE_FAST_INTERP=${WAMR_FAST_INTERP})
target_include_directories(${TARGET_NAME} PRIVATE
    "${WAMR_SRC}/core/iwasm/include"
)

find_package(Threads REQUIRED)
# WAMR's product-mini build produces libvmlib.a (the static runtime).
target_link_libraries(${TARGET_NAME}
    "${WAMR_BUILD}/libvmlib.a"
    Threads::Threads
    ${CMAKE_DL_LIBS}
    m
)
