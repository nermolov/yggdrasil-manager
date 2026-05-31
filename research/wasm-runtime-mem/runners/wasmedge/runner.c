/* runner-wasmedge: WasmEdge host runner for the WASM-runtime memory benchmark.
 *
 * Runs in interpreter mode (no LLVM/AOT). The doc payload is fed on fd 0 via
 * dup2; the framed response is written to fd 1. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>
#include "wasmedge/wasmedge.h"

static void die(const char *msg) {
    fprintf(stderr, "runner-wasmedge: %s\n", msg);
    exit(1);
}

static uint8_t *read_file(const char *path, size_t *out_len) {
    FILE *f = fopen(path, "rb");
    if (!f) { perror(path); return NULL; }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    rewind(f);
    uint8_t *buf = malloc(sz);
    if (!buf) { fclose(f); return NULL; }
    if (fread(buf, 1, sz, f) != (size_t)sz) { fclose(f); free(buf); return NULL; }
    fclose(f);
    *out_len = (size_t)sz;
    return buf;
}

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "usage: runner-wasmedge <module.wasm> <doc.am>\n");
        return 1;
    }

    /* Stage the length-prefixed payload and dup2 it onto fd 0: WasmEdge's WASI
     * stdin maps to the host fd 0. */
    size_t doc_len = 0;
    uint8_t *doc_bytes = read_file(argv[2], &doc_len);
    if (!doc_bytes) die("failed to read doc");
    size_t stdin_len = 4 + doc_len;
    uint8_t *stdin_buf = malloc(stdin_len);
    if (!stdin_buf) die("malloc");
    stdin_buf[0] = (doc_len >> 24) & 0xff;
    stdin_buf[1] = (doc_len >> 16) & 0xff;
    stdin_buf[2] = (doc_len >>  8) & 0xff;
    stdin_buf[3] = (doc_len      ) & 0xff;
    memcpy(stdin_buf + 4, doc_bytes, doc_len);
    free(doc_bytes);
    FILE *tmp = tmpfile();
    if (!tmp) die("tmpfile");
    fwrite(stdin_buf, 1, stdin_len, tmp);
    fflush(tmp);
    rewind(tmp);
    free(stdin_buf);
    if (dup2(fileno(tmp), STDIN_FILENO) < 0) die("dup2 stdin");

    /* Configure interpreter mode and register the WASI host module. */
    WasmEdge_ConfigureContext *cfg = WasmEdge_ConfigureCreate();
    if (!cfg) die("WasmEdge_ConfigureCreate failed");
    WasmEdge_ConfigureAddHostRegistration(cfg, WasmEdge_HostRegistration_Wasi);

    WasmEdge_VMContext *vm = WasmEdge_VMCreate(cfg, NULL);
    WasmEdge_ConfigureDelete(cfg);
    if (!vm) die("WasmEdge_VMCreate failed");

    /* Initialize the WASI module instance (renamed from ImportObject to
     * ModuleInstance in WasmEdge 0.10+). Inherit host stdio. */
    WasmEdge_ModuleInstanceContext *wasi_mod =
        WasmEdge_VMGetImportModuleContext(vm, WasmEdge_HostRegistration_Wasi);
    if (!wasi_mod) die("WASI module instance unavailable");
    const char *wasi_argv[] = { argv[1] };
    WasmEdge_ModuleInstanceInitWASI(wasi_mod, wasi_argv, 1, NULL, 0, NULL, 0);

    WasmEdge_String func_name = WasmEdge_StringCreateByCString("_start");
    WasmEdge_Result res =
        WasmEdge_VMRunWasmFromFile(vm, argv[1], func_name, NULL, 0, NULL, 0);
    WasmEdge_StringDelete(func_name);

    fflush(stdout);

    int rc = 0;
    if (!WasmEdge_ResultOK(res)) {
        /* A clean WASI proc_exit returns a non-OK result carrying the exit
         * code; only a non-zero exit code is a real failure. */
        uint32_t code = WasmEdge_ModuleInstanceWASIGetExitCode(wasi_mod);
        if (code != 0) {
            fprintf(stderr, "runner-wasmedge: %s (wasi exit %u)\n",
                    WasmEdge_ResultGetMessage(res), code);
            rc = 1;
        }
    }

    WasmEdge_VMDelete(vm);
    return rc;
}
