import { spawn, spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const frontendDir = resolve(scriptDir, '..')
const defaultVitePort = 5218
const defaultMaxOldSpaceSizeMb = 256
const defaultMaxSemiSpaceSizeMb = 16
const defaultRssWarnMb = 256
const defaultRssHardLimitMb = 360
const defaultMemoryPollMs = 3000
const nodeExecutable = process.execPath
const ensureNativeScript = resolve(frontendDir, 'scripts', 'ensure-rollup-native.mjs')
const viteEntry = resolve(frontendDir, 'node_modules', 'vite', 'bin', 'vite.js')

function ensureNativeRuntime(env) {
  const result = spawnSync(nodeExecutable, [ensureNativeScript], {
    cwd: frontendDir,
    stdio: 'inherit',
    env,
  })

  if (result.error) {
    throw result.error
  }
  if ((result.status ?? 0) !== 0) {
    throw new Error(`ensure native failed with exit code ${result.status ?? 1}`)
  }
}

function resolveRequestedPort(rawPort, fallbackPort = defaultVitePort) {
  const parsed = Number.parseInt(String(rawPort || '').trim(), 10)
  if (Number.isInteger(parsed) && parsed > 0 && parsed <= 65535) {
    return parsed
  }
  return fallbackPort
}

function resolvePositiveInteger(rawValue, fallbackValue) {
  const parsed = Number.parseInt(String(rawValue || '').trim(), 10)
  if (Number.isInteger(parsed) && parsed > 0) {
    return parsed
  }
  return fallbackValue
}

function resolveNodeArgs(env) {
  const maxOldSpaceSizeMb = resolvePositiveInteger(
    env.FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB,
    defaultMaxOldSpaceSizeMb,
  )
  const maxSemiSpaceSizeMb = resolvePositiveInteger(
    env.FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB,
    defaultMaxSemiSpaceSizeMb,
  )

  const args = [`--max-old-space-size=${maxOldSpaceSizeMb}`]
  if (maxSemiSpaceSizeMb > 0) {
    args.push(`--max-semi-space-size=${maxSemiSpaceSizeMb}`)
  }
  if (String(env.FRONTEND_NODE_HEAP_SNAPSHOT || '').trim() === '1') {
    args.push('--heapsnapshot-near-heap-limit=2')
  }
  args.push(viteEntry)

  return {
    args,
    maxOldSpaceSizeMb,
    maxSemiSpaceSizeMb,
  }
}

function killProcessTree(pid) {
  if (!pid || pid <= 0) {
    return
  }

  if (process.platform === 'win32') {
    spawnSync('taskkill.exe', ['/F', '/T', '/PID', String(pid)], {
      stdio: 'ignore',
    })
    return
  }

  try {
    process.kill(pid, 'SIGTERM')
  } catch {}
}

function readProcessRssMb(pid) {
  if (!pid || pid <= 0) {
    return 0
  }

  if (process.platform === 'win32') {
    const result = spawnSync(
      'powershell.exe',
      [
        '-NoProfile',
        '-Command',
        `$proc = Get-Process -Id ${pid} -ErrorAction SilentlyContinue; if ($proc) { [math]::Round($proc.WorkingSet64 / 1MB, 0) }`,
      ],
      {
        cwd: frontendDir,
        encoding: 'utf8',
      },
    )

    if (result.status !== 0) {
      return 0
    }

    return resolvePositiveInteger(result.stdout, 0)
  }

  const result = spawnSync('ps', ['-o', 'rss=', '-p', String(pid)], {
    cwd: frontendDir,
    encoding: 'utf8',
  })
  if (result.status !== 0) {
    return 0
  }

  const rssKb = resolvePositiveInteger(result.stdout, 0)
  return Math.round(rssKb / 1024)
}

function startMemoryWatcher(child, env) {
  const rssWarnMb = resolvePositiveInteger(env.FRONTEND_NODE_RSS_WARN_MB, defaultRssWarnMb)
  const rssHardLimitMb = resolvePositiveInteger(env.FRONTEND_NODE_RSS_HARD_LIMIT_MB, defaultRssHardLimitMb)
  const pollMs = resolvePositiveInteger(env.FRONTEND_NODE_MEMORY_POLL_MS, defaultMemoryPollMs)
  let warnedAtMb = 0

  const timer = setInterval(() => {
    if (!child.pid || child.exitCode !== null) {
      return
    }

    const rssMb = readProcessRssMb(child.pid)
    if (rssMb <= 0) {
      return
    }

    if (rssMb >= rssWarnMb && (warnedAtMb === 0 || Math.abs(rssMb - warnedAtMb) >= 128)) {
      warnedAtMb = rssMb
      console.warn(`[dev] vite RSS is ${rssMb} MB (warning threshold: ${rssWarnMb} MB)`)
    }

    if (rssHardLimitMb > 0 && rssMb >= rssHardLimitMb) {
      console.error(`[dev] vite RSS reached ${rssMb} MB, exceeding hard limit ${rssHardLimitMb} MB. stopping dev server.`)
      killProcessTree(child.pid)
    }
  }, pollMs)

  timer.unref?.()
  return timer
}

function main() {
  const requestedPort = resolveRequestedPort(process.env.FRONTEND_PORT, defaultVitePort)

  const childEnv = {
    ...process.env,
    FRONTEND_PORT: String(requestedPort),
  }

  ensureNativeRuntime(childEnv)

  const nodeArgs = resolveNodeArgs(childEnv)
  console.log(
    `[dev] starting Vite on http://127.0.0.1:${requestedPort} with --max-old-space-size=${nodeArgs.maxOldSpaceSizeMb} MB --max-semi-space-size=${nodeArgs.maxSemiSpaceSizeMb} MB --rss-hard-limit=${resolvePositiveInteger(childEnv.FRONTEND_NODE_RSS_HARD_LIMIT_MB, defaultRssHardLimitMb)} MB`,
  )

  const child = spawn(nodeExecutable, nodeArgs.args, {
    cwd: frontendDir,
    stdio: 'inherit',
    env: childEnv,
  })
  const memoryWatcher = startMemoryWatcher(child, childEnv)
  let shuttingDown = false

  const shutdown = (exitCode = 0) => {
    if (shuttingDown) {
      return
    }
    shuttingDown = true
    if (memoryWatcher) {
      clearInterval(memoryWatcher)
    }
    if (child.pid && child.exitCode === null) {
      killProcessTree(child.pid)
    }
    process.exit(exitCode)
  }

  const handleSignal = (signal) => {
    console.log(`[dev] received ${signal}, stopping Vite...`)
    shutdown(0)
  }

  process.on('SIGINT', handleSignal)
  process.on('SIGTERM', handleSignal)
  process.on('exit', () => {
    if (memoryWatcher) {
      clearInterval(memoryWatcher)
    }
    if (child.pid && child.exitCode === null) {
      killProcessTree(child.pid)
    }
  })

  child.on('error', (error) => {
    console.error(`[dev] failed to start Vite: ${error instanceof Error ? error.message : String(error)}`)
    shutdown(1)
  })

  child.on('exit', (code, signal) => {
    if (memoryWatcher) {
      clearInterval(memoryWatcher)
    }
    if (signal) {
      console.error(`[dev] vite exited with signal ${signal}`)
      process.exit(1)
      return
    }
    process.exit(code ?? 0)
  })
}

try {
  main()
} catch (error) {
  console.error(`[dev] ${error instanceof Error ? error.message : String(error)}`)
  process.exit(1)
}
