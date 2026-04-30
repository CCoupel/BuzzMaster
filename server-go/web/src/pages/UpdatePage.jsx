import { useState, useEffect } from 'react'
import { useUpdates } from '../hooks/useUpdates'
import { useGame } from '../hooks/GameContext'
import './UpdatePage.css'

export default function UpdatePage() {
    const { gameState } = useGame()
    const {
        loading,
        error,
        updateInfo,
        versions,
        downloadProgress,
        checkForUpdates,
        listUpdates,
        downloadUpdate,
        applyUpdate,
        resetDownloadProgress
    } = useUpdates()

    const [restarting, setRestarting] = useState(false)
    const [showConfirm, setShowConfirm] = useState(null)
    const [expandedNotes, setExpandedNotes] = useState({})

    useEffect(() => {
        checkForUpdates()
        listUpdates()
    }, [checkForUpdates, listUpdates])

    const handleDownload = async (version) => {
        resetDownloadProgress()
        await downloadUpdate(version)
    }

    const handleApply = async (version, path) => {
        const isGameRunning = !['STOPPED', 'NEW_GAME'].includes(gameState?.phase)
        if (isGameRunning) {
            setShowConfirm({ version, path })
        } else {
            await executeApply(version, path)
        }
    }

    const executeApply = async (version, path) => {
        setShowConfirm(null)
        const result = await applyUpdate(version, path)
        if (result?.success) {
            setRestarting(true)
        }
    }

    // Polling effect with proper cleanup
    useEffect(() => {
        if (!restarting) return

        let isActive = true
        let attempts = 0
        const maxAttempts = 30

        const checkServer = async () => {
            if (!isActive) return

            attempts++
            try {
                const response = await fetch('/api/updates/check')
                if (response.ok && isActive) {
                    window.location.reload()
                }
            } catch (err) {
                // Server still down
                if (attempts >= maxAttempts && isActive) {
                    setRestarting(false)
                    alert('Le serveur ne répond plus. Veuillez vérifier manuellement.')
                }
            }
        }

        const interval = setInterval(checkServer, 2000)

        return () => {
            isActive = false
            clearInterval(interval)
        }
    }, [restarting])

    const formatBytes = (bytes) => {
        if (!bytes) return 'N/A'
        const mb = bytes / (1024 * 1024)
        return `${mb.toFixed(1)} MB`
    }

    const formatDate = (dateStr) => {
        if (!dateStr) return 'N/A'
        const date = new Date(dateStr)
        return date.toLocaleDateString('fr-FR', {
            year: 'numeric',
            month: 'long',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        })
    }

    const toggleNotes = (version) => {
        setExpandedNotes(prev => ({
            ...prev,
            [version]: !prev[version]
        }))
    }

    const parseMarkdown = (text) => {
        if (!text) return ''

        return text
            // Escape HTML
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            // Headers ### → <h4>
            .replace(/^### (.+)$/gm, '<h4>$1</h4>')
            // Bold **text** → <strong>
            .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
            // Lists - item → <li>
            .replace(/^- (.+)$/gm, '<li>$1</li>')
            // Wrap consecutive <li> in <ul>
            .replace(/(<li>.*<\/li>\n?)+/g, '<ul>$&</ul>')
            // Line breaks
            .replace(/\n\n/g, '</p><p>')
            .replace(/\n/g, '<br/>')
    }

    const compareVersions = (v1, v2) => {
        const parts1 = v1.split('.').map(Number)
        const parts2 = v2.split('.').map(Number)
        for (let i = 0; i < 3; i++) {
            if (parts1[i] > parts2[i]) return 1
            if (parts1[i] < parts2[i]) return -1
        }
        return 0
    }

    const getVersionStatus = (version) => {
        if (!updateInfo?.current) return 'unknown'
        const cmp = compareVersions(version, updateInfo.current)
        if (cmp === 0) return 'current'
        if (cmp > 0) return 'newer'
        return 'older'
    }

    const getStatusIcon = (status) => {
        switch (status) {
            case 'current': return '✅'
            case 'newer': return '⬆️'
            case 'older': return '⚠️'
            default: return ''
        }
    }

    const isCurrentVersionInList = () => {
        if (!updateInfo?.current || versions.length === 0) return true
        return versions.some(v => v.version === updateInfo.current)
    }

    if (restarting) {
        return (
            <div className="update-page">
                <div className="restart-spinner">
                    <div className="spinner"></div>
                    <h2>Redémarrage en cours...</h2>
                    <p>Le serveur redémarre avec la nouvelle version.</p>
                    <p>Rechargement automatique dans quelques instants...</p>
                </div>
            </div>
        )
    }

    return (
        <div className="update-page">
            <header className="update-header">
                <h1>Mises à jour</h1>
                <button
                    className="btn-refresh"
                    onClick={() => {
                        checkForUpdates()
                        listUpdates()
                    }}
                    disabled={loading}
                >
                    {loading ? 'Actualisation...' : 'Actualiser'}
                </button>
            </header>

            {error && (
                <div className="update-error">
                    <strong>Erreur :</strong> {error}
                </div>
            )}

            {updateInfo && (
                <div className="update-status">
                    <div className="status-card">
                        <h2>État actuel</h2>
                        <div className="status-info">
                            <div className="status-item">
                                <span className="status-label">Version installée :</span>
                                <span className="status-value current-version">{updateInfo.current}</span>
                            </div>
                            <div className="status-item">
                                <span className="status-label">Dernière version :</span>
                                <span className="status-value latest-version">{updateInfo.latest}</span>
                            </div>
                            {updateInfo.update_available ? (
                                <div className="update-available">
                                    <span className="badge badge-update">Mise à jour disponible</span>
                                    {updateInfo.release_url && (
                                        <a
                                            href={updateInfo.release_url}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            className="release-link"
                                        >
                                            Voir sur GitHub
                                        </a>
                                    )}
                                </div>
                            ) : (
                                <div className="update-current">
                                    {isCurrentVersionInList() ? (
                                        <span className="badge badge-success">À jour</span>
                                    ) : (
                                        <span className="badge badge-local">Version locale (non publiée)</span>
                                    )}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}

            <div className="versions-list">
                <h2>Versions disponibles</h2>
                {loading && versions.length === 0 ? (
                    <div className="loading-spinner">Chargement...</div>
                ) : versions.length === 0 ? (
                    <div className="no-versions">Aucune version disponible</div>
                ) : (
                    <div className="versions-table">
                        {versions.map(v => {
                            const isDownloading = downloadProgress?.version === v.version && downloadProgress?.status === 'downloading'
                            const justDownloaded = downloadProgress?.version === v.version && downloadProgress?.status === 'completed'
                            const downloadFailed = downloadProgress?.version === v.version && downloadProgress?.status === 'error'
                            const isExpanded = expandedNotes[v.version]

                            // A version is ready to apply if it was downloaded this session OR was already on disk
                            const isDownloaded = justDownloaded || v.downloaded
                            const applyPath = justDownloaded ? downloadProgress.path : v.local_path

                            const versionStatus = getVersionStatus(v.version)
                            const statusIcon = getStatusIcon(versionStatus)

                            return (
                                <div key={v.version} className={`version-row ${v.current ? 'current' : ''}`}>
                                    <div className="version-main">
                                        <div className="version-info-row">
                                            <div className="version-title">
                                                <span className="version-status-icon">{statusIcon}</span>
                                                <span className="version-number">
                                                    {v.version}
                                                    {v.title && <span className="version-title-text"> - {v.title}</span>}
                                                </span>
                                                {v.current && <span className="badge badge-current">Actuelle</span>}
                                                {!v.current && v.downloaded && !justDownloaded && (
                                                    <span className="badge badge-downloaded">✓ Téléchargé</span>
                                                )}
                                            </div>
                                            <span className="version-date">{formatDate(v.date)}</span>
                                            <span className="version-size">{formatBytes(v.size)}</span>
                                        </div>
                                        <div className="version-actions">
                                            {v.notes && (
                                                <button
                                                    className="btn-toggle-notes"
                                                    onClick={() => toggleNotes(v.version)}
                                                    title="Afficher les notes de version"
                                                >
                                                    <span className={`chevron ${isExpanded ? 'expanded' : ''}`}>▶</span>
                                                    Notes
                                                </button>
                                            )}
                                            {!v.current && (
                                                <>
                                                    {!isDownloaded && !isDownloading && (
                                                        <button
                                                            className="btn-download"
                                                            onClick={() => handleDownload(v.version)}
                                                            disabled={loading}
                                                        >
                                                            Télécharger
                                                        </button>
                                                    )}
                                                    {isDownloading && (
                                                        <div className="download-progress">
                                                            <div className="spinner small"></div>
                                                            <span>Téléchargement...</span>
                                                        </div>
                                                    )}
                                                    {isDownloaded && (
                                                        <button
                                                            className="btn-apply"
                                                            onClick={() => handleApply(v.version, applyPath)}
                                                            disabled={loading}
                                                        >
                                                            Appliquer
                                                        </button>
                                                    )}
                                                    {downloadFailed && (
                                                        <div className="download-error">
                                                            Échec : {downloadProgress.error}
                                                        </div>
                                                    )}
                                                </>
                                            )}
                                        </div>
                                    </div>
                                    {v.notes && isExpanded && (
                                        <div className="version-notes-expanded">
                                            <div
                                                className="release-notes"
                                                dangerouslySetInnerHTML={{ __html: parseMarkdown(v.notes) }}
                                            />
                                        </div>
                                    )}
                                </div>
                            )
                        })}
                    </div>
                )}
            </div>

            {showConfirm && (
                <div className="confirm-modal">
                    <div className="confirm-content">
                        <h2>⚠️ Partie en cours</h2>
                        <p>
                            Une partie est actuellement en cours. Appliquer la mise à jour
                            va redémarrer le serveur et interrompre la partie.
                        </p>
                        <p>
                            <strong>Voulez-vous continuer ?</strong>
                        </p>
                        <div className="confirm-actions">
                            <button
                                className="btn-cancel"
                                onClick={() => setShowConfirm(null)}
                            >
                                Annuler
                            </button>
                            <button
                                className="btn-confirm"
                                onClick={() => executeApply(showConfirm.version, showConfirm.path)}
                            >
                                Appliquer quand même
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
