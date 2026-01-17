import './QuestionPreview.css'

export default function QuestionPreview() {
  return (
    <div className="tv-preview">
      <iframe
        src="/tv?admin=true"
        className="tv-preview-iframe"
        title="Aperçu TV"
      />
      <div className="tv-preview-frame-label">Aperçu TV</div>
    </div>
  )
}
