import { useState, useEffect, useCallback, useRef } from 'react'

const RECONNECT_INTERVAL = 5000

export default function useWebSocket() {
  const [status, setStatus] = useState('disconnected')
  const [gameState, setGameState] = useState({
    phase: 'STOPPED',
    timer: 30,
    totalTime: 30,
    countdownTime: 0,
    gameTime: 0,
    question: null,
    remote: 'GAME',
    backgrounds: [],
    currentBackgroundIndex: 0, // Server-synchronized
    memoryFlippedCards: [], // Server-synchronized flipped Memory cards (max 2)
    memoryMatchedPairs: [], // Server-synchronized matched pair IDs (permanent)
    memoryErrors: 0, // Server-synchronized error count (non-matches)
    qcmInvalidated: [], // Server-synchronized invalidated QCM answers (e.g., ["RED", "YELLOW"])
    virtualPlayerCount: 0, // Server-synchronized virtual player count (ENROLL phase)
    virtualPlayerLimit: 20, // Server-synchronized virtual player limit
    enrollmentActive: false, // Whether enrollment is currently open
    showQRCode: false, // Whether QR code should be displayed on TV
  })
  const [teams, setTeams] = useState({})
  const [bumpers, setBumpers] = useState({})
  const [questions, setQuestions] = useState({})
  const [fsInfo, setFsInfo] = useState(null)
  const [version, setVersion] = useState(null)
  const [clientCounts, setClientCounts] = useState({ admin: 0, tv: 0 })
  const [logs, setLogs] = useState([])

  const wsRef = useRef(null)
  const logCallbackRef = useRef(null)
  const reconnectTimeoutRef = useRef(null)

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`

    setStatus('connecting')
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setStatus('connected')
      sendMessage('HELLO', {})
    }

    ws.onclose = () => {
      setStatus('disconnected')
      wsRef.current = null
      reconnectTimeoutRef.current = setTimeout(connect, RECONNECT_INTERVAL)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        handleMessage(data)
      } catch (error) {
        console.error('Failed to parse message:', error)
      }
    }
  }, [])

  const handleMessage = useCallback((data) => {
    const { ACTION, MSG, FSINFO, VERSION } = data
    console.log('[WS] Received:', ACTION, MSG)

    switch (ACTION) {
      case 'UPDATE':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: MSG.GAME.PHASE || prev.phase,
            timer: MSG.GAME.CURRENT_TIME ?? MSG.GAME.DELAY ?? prev.timer,
            totalTime: MSG.GAME.DELAY ?? prev.totalTime,
            countdownTime: MSG.GAME.COUNTDOWN_TIME ?? prev.countdownTime,
            gameTime: MSG.GAME.TIME ?? prev.gameTime,
            question: MSG.GAME.QUESTION || prev.question,
            remote: MSG.GAME.REMOTE || prev.remote,
            backgrounds: MSG.GAME.backgrounds || prev.backgrounds,
            currentBackgroundIndex: MSG.GAME.CURRENT_BACKGROUND_INDEX ?? prev.currentBackgroundIndex,
            memoryFlippedCards: MSG.GAME.MEMORY_FLIPPED_CARDS || [],
            memoryMatchedPairs: MSG.GAME.MEMORY_MATCHED_PAIRS || [],
            memoryErrors: MSG.GAME.MEMORY_ERRORS || 0,
            qcmInvalidated: MSG.GAME.QCM_INVALIDATED || [],
            virtualPlayerCount: MSG.GAME.VIRTUAL_PLAYER_COUNT ?? prev.virtualPlayerCount,
            virtualPlayerLimit: MSG.GAME.VIRTUAL_PLAYER_LIMIT ?? prev.virtualPlayerLimit,
            enrollmentActive: MSG.GAME.ENROLLMENT_ACTIVE ?? prev.enrollmentActive,
            showQRCode: MSG.GAME.SHOW_QR_CODE ?? prev.showQRCode,
          }))
        }
        if (MSG?.teams) setTeams(MSG.teams)
        if (MSG?.bumpers) setBumpers(MSG.bumpers)
        if (VERSION) setVersion(VERSION)
        break

      case 'UPDATE_TIMER':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: MSG.GAME.PHASE || prev.phase,
            timer: MSG.GAME.CURRENT_TIME ?? prev.timer,
            countdownTime: MSG.GAME.COUNTDOWN_TIME ?? prev.countdownTime,
            gameTime: MSG.GAME.TIME ?? prev.gameTime,
          }))
        }
        break

      case 'START':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: MSG.GAME.PHASE || 'STARTED', // Use server phase (could be COUNTDOWN)
            timer: MSG.GAME.CURRENT_TIME ?? MSG.GAME.DELAY ?? prev.timer,
            totalTime: MSG.GAME.DELAY ?? prev.totalTime,
            countdownTime: MSG.GAME.COUNTDOWN_TIME ?? prev.countdownTime,
            gameTime: MSG.GAME.TIME ?? prev.gameTime,
            question: MSG.GAME.QUESTION || prev.question,
          }))
        }
        break

      case 'STOP':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: 'STOPPED',
            question: MSG.GAME.QUESTION || prev.question,
          }))
        } else {
          setGameState(prev => ({ ...prev, phase: 'STOPPED' }))
        }
        break

      case 'PAUSE':
        setGameState(prev => ({ ...prev, phase: 'PAUSED' }))
        break

      case 'CONTINUE':
        setGameState(prev => ({ ...prev, phase: 'STARTED' }))
        break

      case 'BUMPER':
        if (MSG?.teams) setTeams(MSG.teams)
        if (MSG?.bumpers) setBumpers(MSG.bumpers)
        break

      case 'QUESTIONS':
        console.log('[WS] QUESTIONS handler - MSG:', MSG, 'FSINFO:', FSINFO)
        if (MSG) {
          const questionsMap = {}
          Object.entries(MSG).forEach(([key, value]) => {
            if (key !== 'FSINFO' && value?.ID) {
              questionsMap[value.ID] = value
            }
          })
          console.log('[WS] Parsed questions:', questionsMap)
          setQuestions(questionsMap)
        }
        if (FSINFO) setFsInfo(FSINFO)
        if (VERSION) setVersion(VERSION)
        break

      case 'READY':
        setGameState(prev => ({
          ...prev,
          phase: 'READY',
          question: MSG?.QUESTION || prev.question,
        }))
        break

      case 'REVEAL':
        setGameState(prev => ({ ...prev, phase: 'REVEALED' }))
        break

      case 'REMOTE':
        console.log('[WS] REMOTE handler - MSG.GAME:', MSG?.GAME)
        if (MSG?.GAME?.REMOTE) {
          console.log('[WS] Setting remote to:', MSG.GAME.REMOTE)
          setGameState(prev => ({ ...prev, remote: MSG.GAME.REMOTE }))
        }
        if (MSG?.teams) setTeams(MSG.teams)
        if (MSG?.bumpers) setBumpers(MSG.bumpers)
        break

      case 'CLIENTS':
        if (MSG) {
          setClientCounts({
            admin: MSG.ADMIN_COUNT ?? 0,
            tv: MSG.TV_COUNT ?? 0,
          })
        }
        break

      case 'BACKGROUND_CHANGE':
        if (MSG?.INDEX !== undefined) {
          setGameState(prev => ({
            ...prev,
            currentBackgroundIndex: MSG.INDEX,
          }))
        }
        break

      case 'QCM_HINT':
        // A QCM answer was invalidated - append to the list
        if (MSG?.COLOR) {
          console.log('[WS] QCM_HINT: invalidated color:', MSG.COLOR, 'remaining:', MSG.REMAINING)
          setGameState(prev => ({
            ...prev,
            qcmInvalidated: [...prev.qcmInvalidated, MSG.COLOR],
          }))
        }
        break

      case 'SHOW_QR_CODE':
        console.log('[WS] SHOW_QR_CODE received')
        setGameState(prev => ({
          ...prev,
          showQRCode: true,
          enrollmentActive: true,
        }))
        break

      case 'HIDE_QR_CODE':
        console.log('[WS] HIDE_QR_CODE received')
        setGameState(prev => ({
          ...prev,
          showQRCode: false,
          enrollmentActive: false,
        }))
        break

      case 'PLAYER_CONNECTED':
        console.log('[WS] PLAYER_CONNECTED:', MSG)
        // Player successfully enrolled - state will be updated via UPDATE message
        break

      case 'PLAYER_REJECTED':
        console.log('[WS] PLAYER_REJECTED:', MSG?.REASON)
        // Handle rejection (will be used by EnrollPage)
        break

      case 'PLAYER_ASSIGNED':
        console.log('[WS] PLAYER_ASSIGNED:', MSG)
        // Player assigned to team - state will be updated via UPDATE message
        break

      case 'ENROLLMENT_UPDATE':
        console.log('[WS] ENROLLMENT_UPDATE:', MSG)
        if (MSG) {
          setGameState(prev => ({
            ...prev,
            virtualPlayerCount: MSG.VIRTUAL_PLAYER_COUNT ?? prev.virtualPlayerCount,
            virtualPlayerLimit: MSG.VIRTUAL_PLAYER_LIMIT ?? prev.virtualPlayerLimit,
            enrollmentActive: MSG.ENROLLMENT_ACTIVE ?? prev.enrollmentActive,
          }))
        }
        break

      case 'CONFIG_UPDATE':
        console.log('[WS] CONFIG_UPDATE:', MSG)
        if (MSG?.neon_effect) {
          setGameState(prev => ({
            ...prev,
            neonEffect: MSG.neon_effect,
          }))
        }
        break

      case 'LOG_HISTORY':
        // Receive full log history on subscription
        if (MSG?.entries) {
          setLogs(MSG.entries)
          if (logCallbackRef.current) {
            MSG.entries.forEach(entry => logCallbackRef.current(entry))
          }
        }
        break

      case 'LOG_ENTRY':
        // Receive a single new log entry
        if (MSG) {
          setLogs(prev => [...prev, MSG])
          if (logCallbackRef.current) {
            logCallbackRef.current(MSG)
          }
        }
        break

      default:
        console.log('Unknown action:', ACTION)
    }
  }, [])

  const sendMessage = useCallback((action, msg = {}) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      const message = JSON.stringify({ ACTION: action, MSG: msg })
      console.log('[WS] Sending:', action, msg)
      wsRef.current.send(message)
    } else {
      console.error('WebSocket is not connected')
    }
  }, [])

  // Game actions
  const startGame = useCallback((delay, points) => {
    sendMessage('START', { DELAY: delay, POINTS: points })
  }, [sendMessage])

  const stopGame = useCallback(() => {
    sendMessage('STOP', {})
  }, [sendMessage])

  const pauseGame = useCallback(() => {
    sendMessage('PAUSE', {})
  }, [sendMessage])

  const continueGame = useCallback(() => {
    sendMessage('CONTINUE', {})
  }, [sendMessage])

  const revealAnswer = useCallback(() => {
    sendMessage('REVEAL', {})
  }, [sendMessage])

  const selectQuestion = useCallback((questionId) => {
    sendMessage('READY', { QUESTION: questionId })
  }, [sendMessage])

  const setRemoteDisplay = useCallback((display) => {
    sendMessage('REMOTE', { REMOTE: display })
  }, [sendMessage])

  const updateConfig = useCallback((config) => {
    sendMessage('UPDATE', config)
  }, [sendMessage])

  const deleteQuestion = useCallback((questionId) => {
    sendMessage('DELETE', { ID: questionId })
  }, [sendMessage])

  const setBumperPoints = useCallback((bumperMac, points) => {
    sendMessage('BUMPER_POINTS', { ID: bumperMac, POINTS: points })
  }, [sendMessage])

  const setTeamPoints = useCallback((teamName, points) => {
    sendMessage('TEAM_POINTS', { TEAM: teamName, POINTS: points })
  }, [sendMessage])

  const setClientType = useCallback((type) => {
    sendMessage('SET_CLIENT_TYPE', { TYPE: type })
  }, [sendMessage])

  // Debug: Force transition to READY state (skips PREPARE/PONG wait)
  const forceReady = useCallback(() => {
    sendMessage('FORCE_READY', {})
  }, [sendMessage])

  // Debug: Simulate a button press from a buzzer (for testing)
  const simulateButton = useCallback((bumperMac, button = 'A') => {
    sendMessage('BUTTON', { ID: bumperMac, button })
  }, [sendMessage])

  // Debug: Simulate a PONG response from a buzzer (for testing in PREPARE state)
  const simulatePong = useCallback((bumperMac) => {
    sendMessage('PONG', { ID: bumperMac })
  }, [sendMessage])

  // Memory game: Flip a card
  const flipMemoryCard = useCallback((cardId) => {
    sendMessage('FLIP_MEMORY_CARD', { CARD_ID: cardId })
  }, [sendMessage])

  // VPlayer enrollment: Show QR code
  const showQRCode = useCallback(() => {
    sendMessage('SHOW_QR_CODE', {})
  }, [sendMessage])

  // VPlayer enrollment: Hide QR code
  const hideQRCode = useCallback(() => {
    sendMessage('HIDE_QR_CODE', {})
  }, [sendMessage])

  // VPlayer enrollment: Connect as virtual player
  const connectVirtualPlayer = useCallback((name) => {
    sendMessage('PLAYER_CONNECT', { NAME: name })
  }, [sendMessage])

  // VPlayer enrollment: Set virtual player limit
  const setVirtualPlayerLimit = useCallback((limit) => {
    sendMessage('SET_VIRTUAL_PLAYER_LIMIT', { LIMIT: limit })
  }, [sendMessage])

  // Logs: Subscribe to log updates
  const subscribeLogs = useCallback((callback = null) => {
    logCallbackRef.current = callback
    sendMessage('SUBSCRIBE_LOGS', {})
  }, [sendMessage])

  // Logs: Unsubscribe from log updates
  const unsubscribeLogs = useCallback(() => {
    logCallbackRef.current = null
    sendMessage('UNSUBSCRIBE_LOGS', {})
  }, [sendMessage])

  // Logs: Clear local logs state
  const clearLogs = useCallback(() => {
    setLogs([])
  }, [])

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [connect])

  return {
    status,
    gameState,
    teams,
    bumpers,
    questions,
    fsInfo,
    version,
    clientCounts,
    // Actions
    sendMessage,
    startGame,
    stopGame,
    pauseGame,
    continueGame,
    revealAnswer,
    selectQuestion,
    setRemoteDisplay,
    updateConfig,
    deleteQuestion,
    setBumperPoints,
    setTeamPoints,
    setClientType,
    forceReady,
    simulateButton,
    simulatePong,
    flipMemoryCard,
    // VPlayer enrollment
    showQRCode,
    hideQRCode,
    connectVirtualPlayer,
    setVirtualPlayerLimit,
    // Logs
    logs,
    subscribeLogs,
    unsubscribeLogs,
    clearLogs,
  }
}
