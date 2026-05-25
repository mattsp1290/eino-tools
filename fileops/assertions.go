package fileops

import "github.com/cloudwego/eino/components/tool"

var (
	_ tool.InvokableTool = (*ReadTool)(nil)
	_ tool.InvokableTool = (*WriteTool)(nil)
	_ tool.InvokableTool = (*EditTool)(nil)
	_ tool.InvokableTool = (*ListTool)(nil)
)
