import { chromium } from 'playwright-core'

const baseURL = process.env.GOLF_BASE_URL ?? 'http://localhost:8080'
const adminPassword = process.env.GOLF_ADMIN_PASSWORD

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

  if (adminPassword) {
    await page.goto(`${baseURL}/login`, { waitUntil: 'networkidle' })
    await page.getByLabel('Admin password').fill(adminPassword)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL(`${baseURL}/`)
  }

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
    const renameButton = page.getByTitle('Rename course').first()
    await renameButton.click()
    if (!(await page.getByRole('dialog', { name: 'Rename course' }).isVisible())) {
      failures.push(`${viewport.name}: rename course dialog did not open`)
    }
    const focusIsInsideDialog = await page.evaluate(() => document.activeElement?.closest('[role="dialog"]') !== null)
    if (!focusIsInsideDialog) {
      failures.push(`${viewport.name}: focus did not move inside the rename course dialog`)
    }
    await page.keyboard.press('Escape')
    if (await page.getByRole('dialog', { name: 'Rename course' }).count() !== 0) {
      failures.push(`${viewport.name}: Escape did not close the rename course dialog`)
    }
    if (!(await renameButton.evaluate((button) => button === document.activeElement))) {
      failures.push(`${viewport.name}: focus did not return to the rename course button`)
    }
    await page.locator('.tee-summary').first().click()
    await page.getByRole('button', { name: 'Edit tee' }).first().click()
    if (!(await page.getByRole('dialog', { name: 'Edit tee' }).isVisible())) {
      failures.push(`${viewport.name}: edit tee dialog did not open`)
    }
    const courseRatingField = page.getByLabel('Course Rating', { exact: true })
    if (await courseRatingField.count() !== 1) {
      failures.push(`${viewport.name}: Course Rating label is not associated with exactly one field`)
    } else {
      if (await courseRatingField.getAttribute('role') !== 'spinbutton') {
        failures.push(`${viewport.name}: Course Rating does not use the number field spinbutton`)
      }
      await courseRatingField.fill('68.1')
      await courseRatingField.press('Tab')
      await courseRatingField.press('ArrowUp')
      if (await courseRatingField.getAttribute('aria-valuenow') !== '68.2') {
        failures.push(`${viewport.name}: Course Rating number field did not accept a decimal value and ArrowUp step`)
      }
    }
    const firstParField = page.getByLabel('Hole 1 par', { exact: true })
    if (await firstParField.count() !== 1 || await firstParField.getAttribute('role') !== 'spinbutton') {
      failures.push(`${viewport.name}: hole par input does not use an accessible number field`)
    }
    const dialogOverflow = await page.evaluate(() => ({
      width: document.documentElement.scrollWidth,
      viewport: document.documentElement.clientWidth,
      offenders: [...document.querySelectorAll('*')]
        .map((element) => ({
          element: `${element.tagName.toLowerCase()}.${element.className}`,
          width: element.scrollWidth,
          client: element.clientWidth,
          right: Math.round(element.getBoundingClientRect().right),
        }))
        .filter((element) => element.width > element.client + 1 || element.right > document.documentElement.clientWidth + 1)
        .sort((a, b) => Math.max(b.width - b.client, b.right - document.documentElement.clientWidth)
          - Math.max(a.width - a.client, a.right - document.documentElement.clientWidth))
        .slice(0, 8),
    }))
    if (dialogOverflow.width > dialogOverflow.viewport + 1) {
      failures.push(`${viewport.name}: open dialog width ${dialogOverflow.width} exceeds viewport ${dialogOverflow.viewport}; offenders ${JSON.stringify(dialogOverflow.offenders)}`)
    }
    await page.screenshot({ path: `/tmp/golf-course-edit-${viewport.name}.png`, fullPage: true })
    await page.getByTitle('Close', { exact: true }).click()
  }

  await page.goto(`${baseURL}/players`, { waitUntil: 'networkidle' })
  const editPlayerButtons = await page.getByTitle('Edit player').count()
  if (editPlayerButtons > 0) {
    await page.getByTitle('Edit player').first().click()
    const officialHandicapField = page.getByLabel('Official Handicap Index', { exact: true })
    const categorySelect = page.getByLabel('Handicap category', { exact: true })
    if (await categorySelect.count() !== 1 || await categorySelect.getAttribute('role') !== 'combobox') {
      failures.push(`${viewport.name}: Handicap category does not use an accessible select`)
    } else {
      await categorySelect.click()
      if (await page.getByRole('option').count() !== 2) {
        failures.push(`${viewport.name}: Handicap category select did not show both options`)
      }
      await page.screenshot({ path: `/tmp/golf-player-category-${viewport.name}.png`, fullPage: true })
      await page.keyboard.press('Escape')
    }
    if (await officialHandicapField.count() !== 1) {
      failures.push(`${viewport.name}: Official Handicap Index label is not associated with exactly one field`)
    } else {
      await officialHandicapField.fill('24.1')
      await officialHandicapField.press('Tab')
      if (await officialHandicapField.getAttribute('aria-valuenow') !== '24.1') {
        failures.push(`${viewport.name}: Official Handicap Index did not accept a decimal value`)
      }
      await officialHandicapField.fill('')
      await officialHandicapField.press('Tab')
      const clearedValue = await officialHandicapField.inputValue()
      const clearedAriaValue = await officialHandicapField.getAttribute('aria-valuenow')
      const cleared = clearedValue === '' && clearedAriaValue === null
      if (!cleared) {
        failures.push(`${viewport.name}: optional Official Handicap Index could not be cleared (value "${clearedValue}", aria-valuenow "${clearedAriaValue}")`)
      }
    }
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
  const scoreCount = await page.locator('.nine-scores .score-mark').count()
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
    const displayedNetRows = await page.locator('.nine-net').count()
    if (displayedNetRows !== expectedNetRows * 2) {
      failures.push(`${viewport.name}: displayed ${displayedNetRows} default net rows, expected ${expectedNetRows * 2}`)
    }
    const visibleNines = await page.locator('.nine-card:visible').count()
    const expectedNines = netRound.participants.length * 2
    if (visibleNines !== expectedNines) {
      failures.push(`${viewport.name}: displayed ${visibleNines} nine-hole cards, expected ${expectedNines}`)
    }
    const firstScorecardLayout = await page.locator('.round-scorecard').first().evaluate((scorecard) => {
      const cards = [...scorecard.querySelectorAll('.nine-card')]
      return cards.slice(0, 2).map((card) => {
        const bounds = card.getBoundingClientRect()
        return { left: bounds.left, top: bounds.top }
      })
    })
    const ninesAreSideBySide = firstScorecardLayout.length === 2
      && Math.abs(firstScorecardLayout[0].top - firstScorecardLayout[1].top) < 1
      && firstScorecardLayout[1].left > firstScorecardLayout[0].left
    if (viewport.name === 'desktop' && !ninesAreSideBySide) {
      failures.push(`${viewport.name}: front and back nines are not side by side`)
    }
    if (viewport.name === 'phone' && ninesAreSideBySide) {
      failures.push(`${viewport.name}: front and back nines should stack`)
    }
    const overflowingNines = await page.locator('.nine-card:visible').evaluateAll((cards) =>
      cards.filter((card) => card.scrollWidth > card.clientWidth + 1).length)
    if (overflowingNines > 0) {
      failures.push(`${viewport.name}: ${overflowingNines} nine-hole cards overflow horizontally`)
    }
    await page.screenshot({ path: `/tmp/golf-round-net-scores-${viewport.name}.png`, fullPage: true })
    const netSwitch = page.getByRole('switch', { name: 'Show net' })
    if (await netSwitch.count() !== 1 || await netSwitch.getAttribute('aria-checked') !== 'true') {
      failures.push(`${viewport.name}: net score switch was not on by default`)
    }
    await netSwitch.click()
    const hiddenNetRows = await page.locator('.nine-net').count()
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
