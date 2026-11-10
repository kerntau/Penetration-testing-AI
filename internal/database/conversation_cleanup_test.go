package database

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestDeleteConversationRemovesEinoScopedDirs(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "conversations.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	plantaskBase := filepath.Join(tmp, "skills", ".eino", "plantask")
	checkpointBase := filepath.Join(tmp, "eino-checkpoints")
	reductionBase := filepath.Join(tmp, "reduction")
	workspaceBase := filepath.Join(tmp, "workspace")
	db.SetEinoConversationDirs(plantaskBase, checkpointBase, reductionBase, workspaceBase)

	conv, err := db.CreateConversation("cleanup test", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	convID := conv.ID
	seg := sanitizeConversationPathSegment(convID)
	for _, base := range []struct {
		root string
		file string
	}{
		{db.conversationArtifactsDir, "transcript.txt"},
		{plantaskBase, "task-1.json"},
		{checkpointBase, "runner-deep.ckpt"},
		{filepath.Join(reductionBase, "conversations"), "tool-output.txt"},
		{filepath.Join(workspaceBase, "conversations"), "page.html"},
	} {
		dir := filepath.Join(base.root, seg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, base.file), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", base.file, err)
		}
	}

	if err := db.DeleteConversation(convID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	for _, base := range []string{db.conversationArtifactsDir, plantaskBase, checkpointBase, filepath.Join(reductionBase, "conversations"), filepath.Join(workspaceBase, "conversations")} {
		dir := filepath.Join(base, seg)
		if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
			t.Fatalf("expected removed dir %s, stat err=%v", dir, statErr)
		}
	}
}

// chat_uploads 比其他产物多一层日期目录（chat_uploads/<日期>/<会话ID>），
// 历史上删除会话只清了数据库行（ON DELETE CASCADE），磁盘文件会永久残留。
