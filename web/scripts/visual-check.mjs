import { chromium } from 'playwright-core'

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
    await page.goto(`http://localhost:8080${path}`, { waitUntil: 'networkidle' })
    const overflow = await page.evaluate(() => ({
      width: document.documentElement.scrollWidth,
      viewport: document.documentElement.clientWidth,
    }))
    if (overflow.width > overflow.viewport + 1) {
      failures.push(`${viewport.name} ${path}: page width ${overflow.width} exceeds viewport ${overflow.viewport}`)
    }
  }

  await page.goto('http://localhost:8080/', { waitUntil: 'networkidle' })
  await page.screenshot({ path: `/tmp/golf-overview-${viewport.name}.png`, fullPage: true })

  await page.goto('http://localhost:8080/courses', { waitUntil: 'networkidle' })
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

  await page.goto('http://localhost:8080/players/1', { waitUntil: 'networkidle' })
  const expectedFlags = await page.evaluate(async () => {
    const detail = await fetch('/api/players/1').then((response) => response.json())
    return detail.rounds.flatMap((round) => round.participants).filter((participant) => participant.playerId === 1 && participant.counting).length
  })
  const displayedFlags = await page.locator('.counting-flag:visible').count()
  if (displayedFlags !== expectedFlags) {
    failures.push(`${viewport.name}: displayed ${displayedFlags} counting flags, expected ${expectedFlags}`)
  }
  await page.screenshot({ path: `/tmp/golf-player-counting-${viewport.name}.png`, fullPage: true })

  await page.goto('http://localhost:8080/rounds/1', { waitUntil: 'networkidle' })
  const scoreCount = await page.locator('.scorecard-row.scores .score-mark').count()
  const playerCount = await page.locator('.score-section').count()
  if (scoreCount !== playerCount * 18) {
    failures.push(`${viewport.name}: rendered ${scoreCount} score marks for ${playerCount} scorecards`)
  }
  await page.screenshot({ path: `/tmp/golf-round-scores-${viewport.name}.png`, fullPage: true })

  await page.goto('http://localhost:8080/rounds/new', { waitUntil: 'networkidle' })
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
  await page.close()
}

await browser.close()
if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}
console.log('Responsive browser checks passed.')
