/* runner-toywasm: toywasm host runner for the WASM-runtime memory benchmark. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* toywasm public API */
#include "toywasm/toywasm_config.h"
#include "toywasm/exec.h"
#include "toywasm/load_context.h"
#include "toywasm/module.h"
#include "toywasm/wasi.h"
#include "toywasm/xlog.h"

static void die(const char *msg) {
    fprintf(stderr, "runner-toywasm: %s\n", msg);
    exit(1);
}

static uint8_t *read_file(const char *path, size_t *out_len) {
    FILE *f = fopen(path, "rb");
    if (!f) { perror(path); return NULL; }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    rewind(f);
    uint8_t *buf = malloc(sz);
    fread(buf, 1, sz, f);
    fclose(f);
    *out_len = (size_t)sz;
    return buf;
}

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "usage: runner-toywasm <module.wasm> <doc.am>\n");
        return 1;
    }

    size_t wasm_len = 0;
    uint8_t *wasm_bytes = read_file(argv[1], &wasm_len);
    if (!wasm_bytes) die("failed to read wasm module");

    struct module *mod = NULL;
    struct load_context lctx;
    load_context_init(&lctx);

    int ret = module_load(wasm_bytes, wasm_len, &lctx, &mod);
    if (ret != 0) { fprintf(stderr, "runner-toywasm: module_load: %d\n", ret); return 1; }
    load_context_clear(&lctx);

    struct wasi_instance *wasi = NULL;
    const char *wasi_argv[] = { argv[1], argv[2] };
    ret = wasi_instance_create(mod, 2, wasi_argv, 0, NULL, 0, NULL, &wasi);
    if (ret != 0) { fprintf(stderr, "runner-toywasm: wasi_instance_create: %d\n", ret); return 1; }

    struct instance *inst = NULL;
    ret = instance_create(mod, &inst, NULL, wasi_instance_resolver(wasi), NULL);
    if (ret != 0) { fprintf(stderr, "runner-toywasm: instance_create: %d\n", ret); return 1; }

    struct exec_context ectx;
    exec_context_init(&ectx, inst);

    ret = instance_execute_func_nocheck(&ectx, wasi_start_func(wasi), NULL, NULL);
    uint32_t wasi_exit = 0;
    if (ret != 0) {
        if (wasi_is_user_error(wasi, ret, &wasi_exit)) {
            ret = (int)wasi_exit;
        } else {
            fprintf(stderr, "runner-toywasm: exec: %d\n", ret);
        }
    }

    exec_context_clear(&ectx);
    instance_destroy(inst);
    wasi_instance_destroy(wasi);
    module_destroy(mod);
    free(wasm_bytes);
    return ret;
}
