import { chromium } from 'playwright-core'

const baseURL = process.env.GOLF_BASE_URL ?? 'http://localhost:8080'

const browser = await chromium.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  headless: true,
})

const failures = []
for (const viewport of [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'phone', width: 390, height: 844 },
]) {
  const page = await browser.newPage({ viewport: { width: viewport.width, height: viewport.height } })
  page.on('console', (message) => {
    if (message.type() === 'error') failures.push(`${viewport.name} console: ${message.text()}`)
  })
  page.on('pageerror', (error) => failures.push(`${viewport.name} page: ${error.message}`))

  for (const path of ['/', '/players', '/courses', '/rounds', '/rounds/new', '/rounds/1', '/rounds/1/edit']) {
    await page.goto(`${baseURL}${path}`, { waitUntil: 'networkidle' })
    const overflow = await page.evaluate(() => ({
      width: document.documentElement.scrollWidth,
      viewport: document.documentElement.clientWidth,
    }))
    if (overflow.width > overflow.viewport + 1) {
      failures.push(`${viewport.name} ${path}: page width ${overflow.width} exceeds viewport ${overflow.viewport}`)
    }
  }

  await page.goto(`${baseURL}/`, { waitUntil: 'networkidle' })
  await page.screenshot({ path: `/tmp/golf-overview-${viewport.name}.png`, fullPage: true })

  await page.goto(`${baseURL}/rounds`, { waitUntil: 'networkidle' })
  const firstRound = await page.evaluate(async () => {
    const rounds = await fetch('/api/rounds?limit=200').then((response) => response.json())
    return rounds[0]
  })
  const historySummaries = page.locator('.history-main span')
  if (firstRound && await historySummaries.count() > 0) {
    const expectedSummary = firstRound.participants
      .map((participant) => `${participant.playerName}: ${participant.gross}(${Math.round(participant.netScore)})`)
      .join(' · ')
    const displayedSummary = await historySummaries.first().innerText()
    if (displayedSummary !== expectedSummary) {
      failures.push(`${viewport.name}: round summary "${displayedSummary}", expected "${expectedSummary}"`)
    }
  }
  await page.screenshot({ path: `/tmp/golf-round-history-${viewport.name}.png`, fullPage: true })

  await page.goto(`${baseURL}/courses`, { waitUntil: 'networkidle' })
  const renameButtons = await page.getByTitle('Rename course').count()
  if (renameButtons > 0) {
    await page.getByTitle('Rename course').first().click()
    if (!(await page.getByRole('dialog', { name: 'Rename course' }).isVisible())) {
      failures.push(`${viewport.name}: rename course dialog did not open`)
    }
    await page.getByTitle('Close', { exact: true }).click()
    await page.locator('.tee-summary').first().click()
    await page.getByRole('button', { name: 'Edit tee' }).first().click()
    if (!(await page.getByRole('dialog', { name: 'Edit tee' }).isVisible())) {
      failures.push(`${viewport.name}: edit tee dialog did not open`)
    }
    await page.screenshot({ path: `/tmp/golf-course-edit-${viewport.name}.png`, fullPage: true })
    await page.getByTitle('Close', { exact: true }).click()
  }

  await page.goto(`${baseURL}/players/1`, { waitUntil: 'networkidle' })
  const expectedFlags = await page.evaluate(async () => {
    const detail = await fetch('/api/players/1').then((response) => response.json())
    return detail.rounds.flatMap((round) => round.participants).filter((participant) => participant.playerId === 1 && participant.counting).length
  })
  const displayedFlags = await page.locator('.counting-flag:visible').count()
  if (displayedFlags !== expectedFlags) {
    failures.push(`${viewport.name}: displayed ${displayedFlags} counting flags, expected ${expectedFlags}`)
  }
  await page.screenshot({ path: `/tmp/golf-player-counting-${viewport.name}.png`, fullPage: true })

  await page.goto(`${baseURL}/rounds/1`, { waitUntil: 'networkidle' })
  const scoreCount = await page.locator('.scorecard-row.scores .score-mark').count()
  const playerCount = await page.locator('.score-section').count()
  if (scoreCount !== playerCount * 18) {
    failures.push(`${viewport.name}: rendered ${scoreCount} score marks for ${playerCount} scorecards`)
  }
  await page.screenshot({ path: `/tmp/golf-round-scores-${viewport.name}.png`, fullPage: true })

  const netRound = await page.evaluate(async () => {
    const rounds = await fetch('/api/rounds?limit=200').then((response) => response.json())
    return rounds.find((round) => round.participants.some((participant) => participant.handicapUsed === null))
  })
  if (netRound) {
    await page.goto(`${baseURL}/rounds/${netRound.id}`, { waitUntil: 'networkidle' })
    const expectedNetRows = netRound.participants.length
    const displayedNetRows = await page.locator('.scorecard-row.net-scores').count()
    if (displayedNetRows !== expectedNetRows) {
      failures.push(`${viewport.name}: displayed ${displayedNetRows} default net rows, expected ${expectedNetRows}`)
    }
    const visibleMobileNines = await page.locator('.mobile-nine:visible').count()
    const expectedMobileNines = viewport.name === 'phone' ? netRound.participants.length * 2 : 0
    if (visibleMobileNines !== expectedMobileNines) {
      failures.push(`${viewport.name}: displayed ${visibleMobileNines} mobile nine-hole cards, expected ${expectedMobileNines}`)
    }
    if (viewport.name === 'phone') {
      const overflowingNines = await page.locator('.mobile-nine:visible').evaluateAll((cards) =>
        cards.filter((card) => card.scrollWidth > card.clientWidth + 1).length)
      if (overflowingNines > 0) {
        failures.push(`${viewport.name}: ${overflowingNines} mobile nine-hole cards overflow horizontally`)
      }
    }
    await page.screenshot({ path: `/tmp/golf-round-net-scores-${viewport.name}.png`, fullPage: true })
    await page.getByRole('button', { name: 'Hide net' }).click()
    const hiddenNetRows = await page.locator('.scorecard-row.net-scores').count()
    if (hiddenNetRows !== 0) {
      failures.push(`${viewport.name}: displayed net rows after the toggle was disabled`)
    }
  }

  await page.goto(`${baseURL}/rounds/new`, { waitUntil: 'networkidle' })
  const addToRoundButtons = await page.getByRole('button', { name: 'Add to round' }).count()
  if (addToRoundButtons > 0) {
    await page.getByRole('button', { name: 'Add to round' }).click()
    const mobileVisible = await page.locator('.mobile-score-entry').isVisible()
    const desktopVisible = await page.locator('.desktop-scorecard').isVisible()
    if (viewport.name === 'phone' && (!mobileVisible || desktopVisible)) {
      failures.push('phone score entry did not switch to the mobile one-hole layout')
    }
    if (viewport.name === 'desktop' && (!desktopVisible || mobileVisible)) {
      failures.push('desktop score entry did not show the full scorecard')
    }
    await page.screenshot({ path: `/tmp/golf-score-entry-${viewport.name}.png`, fullPage: true })
  }
  await page.close()
}

await browser.close()
if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}
console.log('Responsive browser checks passed.')
