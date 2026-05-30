/* runner-wasm3: wasm3 host runner for the WASM-runtime memory benchmark. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>

/* wasm3 public API */
#include "wasm3.h"
#include "m3_env.h"

static void die(const char *msg) {
    fprintf(stderr, "runner-wasm3: %s\n", msg);
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
    fread(buf, 1, sz, f);
    fclose(f);
    *out_len = (size_t)sz;
    return buf;
}

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "usage: runner-wasm3 <module.wasm> <doc.am>\n");
        return 1;
    }

    size_t wasm_len = 0, doc_len = 0;
    uint8_t *wasm_bytes = read_file(argv[1], &wasm_len);
    uint8_t *doc_bytes  = read_file(argv[2], &doc_len);
    if (!wasm_bytes || !doc_bytes) die("failed to read input files");

    /* Build length-prefixed stdin: [4-byte BE length][doc bytes] */
    size_t stdin_len = 4 + doc_len;
    uint8_t *stdin_buf = malloc(stdin_len);
    if (!stdin_buf) die("malloc");
    stdin_buf[0] = (doc_len >> 24) & 0xff;
    stdin_buf[1] = (doc_len >> 16) & 0xff;
    stdin_buf[2] = (doc_len >>  8) & 0xff;
    stdin_buf[3] = (doc_len      ) & 0xff;
    memcpy(stdin_buf + 4, doc_bytes, doc_len);
    free(doc_bytes);

    /* Feed the length-prefixed payload to the module via real fd 0: wasm3's
     * WASI fd_read reads the host's stdin, so we stage the bytes in a tmpfile
     * and dup2 it onto STDIN_FILENO before running _start. */
    FILE *tmp = tmpfile();
    if (!tmp) die("tmpfile");
    fwrite(stdin_buf, 1, stdin_len, tmp);
    fflush(tmp);
    rewind(tmp);
    free(stdin_buf);
    if (dup2(fileno(tmp), STDIN_FILENO) < 0) die("dup2 stdin");

    IM3Environment env = m3_NewEnvironment();
    IM3Runtime      rt  = m3_NewRuntime(env, 65536, NULL);

    IM3Module mod;
    M3Result  res = m3_ParseModule(env, &mod, wasm_bytes, (uint32_t)wasm_len);
    if (res) { fprintf(stderr, "runner-wasm3: parse: %s\n", res); return 1; }

    res = m3_LoadModule(rt, mod);
    if (res) { fprintf(stderr, "runner-wasm3: load: %s\n", res); return 1; }

    /* Link WASI — wasm3 provides wasi_snapshot_preview1 linkage */
    res = m3_LinkWASI(mod);
    if (res) { fprintf(stderr, "runner-wasm3: link wasi: %s\n", res); return 1; }

    IM3Function f;
    res = m3_FindFunction(&f, rt, "_start");
    if (res) { fprintf(stderr, "runner-wasm3: find _start: %s\n", res); return 1; }

    res = m3_CallV(f);
    /* A clean WASI proc_exit surfaces as the m3Err_trapExit trap; wasm3's
     * uvwasi layer does not expose the exit code, so we treat that trap as a
     * normal exit. The harness validates correctness via the framed stdout. */
    if (res && strcmp(res, m3Err_trapExit) != 0) {
        fprintf(stderr, "runner-wasm3: call _start: %s\n", res);
        return 1;
    }

    fflush(stdout);
    free(wasm_bytes);
    return 0;
}
