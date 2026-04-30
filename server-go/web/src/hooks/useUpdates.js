import { useState, useCallback } from 'react'

/**
 * Hook pour la gestion des mises à jour automatiques
 * Expose les méthodes pour vérifier, télécharger et appliquer les mises à jour
 */
export function useUpdates() {
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState(null)
    const [updateInfo, setUpdateInfo] = useState(null)
    const [versions, setVersions] = useState([])
    const [downloadProgress, setDownloadProgress] = useState(null)

    /**
     * Vérifie si une mise à jour est disponible
     * GET /api/updates/check
     */
    const checkForUpdates = useCallback(async () => {
        setLoading(true)
        setError(null)
        try {
            const response = await fetch('/api/updates/check')
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }
            const data = await response.json()
            setUpdateInfo(data)
            return data
        } catch (err) {
            setError(err.message)
            return null
        } finally {
            setLoading(false)
        }
    }, [])

    /**
     * Liste toutes les versions disponibles
     * GET /api/updates
     */
    const listUpdates = useCallback(async () => {
        setLoading(true)
        setError(null)
        try {
            const response = await fetch('/api/updates')
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }
            const data = await response.json()
            setVersions(data.versions || [])
            return data.versions || []
        } catch (err) {
            setError(err.message)
            return []
        } finally {
            setLoading(false)
        }
    }, [])

    /**
     * Télécharge une version spécifique
     * POST /api/updates/download
     */
    const downloadUpdate = useCallback(async (version) => {
        setLoading(true)
        setError(null)
        setDownloadProgress({ version, status: 'downloading', percent: 0 })

        try {
            const response = await fetch('/api/updates/download', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ version })
            })

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()

            if (data.success) {
                setDownloadProgress({
                    version,
                    status: 'completed',
                    percent: 100,
                    path: data.path,
                    size: data.size,
                    checksum: data.checksum
                })
            } else {
                throw new Error(data.error || 'Download failed')
            }

            return data
        } catch (err) {
            setError(err.message)
            setDownloadProgress({ version, status: 'error', error: err.message })
            return null
        } finally {
            setLoading(false)
        }
    }, [])

    /**
     * Applique une mise à jour téléchargée
     * POST /api/updates/apply
     * Le serveur redémarre automatiquement
     */
    const applyUpdate = useCallback(async (version) => {
        setLoading(true)
        setError(null)

        try {
            const response = await fetch('/api/updates/apply', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ version })
            })

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()
            return data
        } catch (err) {
            setError(err.message)
            return null
        } finally {
            setLoading(false)
        }
    }, [])

    /**
     * Réinitialise l'état de progression du téléchargement
     */
    const resetDownloadProgress = useCallback(() => {
        setDownloadProgress(null)
    }, [])

    return {
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
    }
}
