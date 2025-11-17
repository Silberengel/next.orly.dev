#!/usr/bin/env node

const fs = require('fs')

const filePath = '/home/orly/cs-policy-output.txt'

const fileExists = fs.existsSync(filePath)

if (fileExists) {
    fs.appendFileSync(filePath, `${Date.now()}: Hey there!\n`)
} else {
    fs.writeFileSync(filePath, `${Date.now()}: Hey there!\n`)
}
