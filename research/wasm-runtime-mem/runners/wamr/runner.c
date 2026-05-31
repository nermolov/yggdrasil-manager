/* runner-wamr: WAMR host runner for the WASM-runtime memory benchmark.
 * Compile with -DUSE_FAST_INTERP=1 for the fast-interpreter variant. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>
#include "wasm_export.h"

static void die(const char *msg) {
    fprintf(stderr, "runner-wamr: %s\n", msg);
    exit(1);
}

static uint8_t *read_file(const char *path, uint32_t *out_len) {
    FILE *f = fopen(path, "rb");
    if (!f) { perror(path); return NULL; }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    rewind(f);
    uint8_t *buf = (uint8_t *)malloc(sz);
    fread(buf, 1, sz, f);
    fclose(f);
    *out_len = (uint32_t)sz;
    return buf;
}

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "usage: runner-wamr <module.wasm> <doc.am>\n");
        return 1;
    }

    uint32_t wasm_len = 0, doc_len = 0;
    uint8_t *wasm_bytes = read_file(argv[1], &wasm_len);
    uint8_t *doc_bytes  = read_file(argv[2], &doc_len);
    if (!wasm_bytes || !doc_bytes) die("failed to read input files");

    /* Build length-prefixed stdin payload */
    uint32_t stdin_len = 4 + doc_len;
    uint8_t *stdin_buf = (uint8_t *)malloc(stdin_len);
    stdin_buf[0] = (doc_len >> 24) & 0xff;
    stdin_buf[1] = (doc_len >> 16) & 0xff;
    stdin_buf[2] = (doc_len >>  8) & 0xff;
    stdin_buf[3] = (doc_len      ) & 0xff;
    memcpy(stdin_buf + 4, doc_bytes, doc_len);
    free(doc_bytes);

    /* WAMR's libc-wasi maps WASI fd 0 to the host stdin (fd 0). Stage the
     * payload in a tmpfile and dup2 it onto STDIN_FILENO before running. */
    FILE *tmp = tmpfile();
    if (!tmp) die("tmpfile");
    fwrite(stdin_buf, 1, stdin_len, tmp);
    fflush(tmp);
    rewind(tmp);
    free(stdin_buf);
    if (dup2(fileno(tmp), STDIN_FILENO) < 0) die("dup2 stdin");

    RuntimeInitArgs init_args;
    memset(&init_args, 0, sizeof(init_args));
    init_args.mem_alloc_type = Alloc_With_System_Allocator;

    if (!wasm_runtime_full_init(&init_args))
        die("wasm_runtime_full_init failed");

    char err[128];
    wasm_module_t mod = wasm_runtime_load(wasm_bytes, wasm_len, err, sizeof(err));
    if (!mod) { fprintf(stderr, "runner-wamr: load: %s\n", err); return 1; }

    /* WASI args must be configured on the module *before* instantiation.
     * This default-stdio variant inherits the host fds 0/1/2, so the module
     * reads our dup2'd stdin and writes the framed response to our stdout. */
    char *wasi_argv[] = { argv[1] };
    wasm_runtime_set_wasi_args(mod, NULL, 0, NULL, 0, NULL, 0, wasi_argv, 1);

    wasm_module_inst_t inst = wasm_runtime_instantiate(mod, 65536, 65536, err, sizeof(err));
    if (!inst) { fprintf(stderr, "runner-wamr: instantiate: %s\n", err); return 1; }

    int ret = 0;
    if (!wasm_application_execute_main(inst, 0, NULL)) {
        const char *ex = wasm_runtime_get_exception(inst);
        /* A clean WASI proc_exit(0) is reported as an exception string; only
         * a non-zero WASI exit code is a real failure. */
        uint32_t code = wasm_runtime_get_wasi_exit_code(inst);
        if (code != 0) {
            fprintf(stderr, "runner-wamr: exec: %s (wasi exit %u)\n",
                    ex ? ex : "(none)", code);
            ret = 1;
        }
    }

    fflush(stdout);
    wasm_runtime_deinstantiate(inst);
    wasm_runtime_unload(mod);
    wasm_runtime_destroy();
    free(wasm_bytes);
    return ret;
}
