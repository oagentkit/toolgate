package policy

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// DefaultPolicy 内置基座：auto 代批至 L2、full 代批至 L3（L4 恒人工双确认）。
func DefaultPolicy() *Policy {
	return &Policy{AutoMaxRisk: 2, FullMaxRisk: 3}
}

// LoadOverrides 读 <fsys>/<path> 外置策略文件（yaml 根键 operation_policy）。
// 缺文件 → 内置基座（nil error）；读取/解析失败 → error（fail-fast，启动即败，
// 宁拒不启动不带错跑）。阈值三档语义（oagentkit/toolgate#44）：
//
//	缺省（未写该键）  → 继承基座；
//	正数             → 调低（≤基座才有效，调高=扩权须改基座走评审 → fail-fast）；
//	0                → 禁用代批（auto/full 判定面自然不命中——此前 <=0 钳回基座，
//	                   「调低」与「禁用」被一并吞掉，配置面无禁用表达）；
//	负数 / 超基座     → 解析 error（fail-fast）。
//
// 区分「未写」与「显式 0」靠 *int 影子解码（yaml 缺键零值也是 0，裸 int 分不出）。
// path 由宿主传入（如领域包布局约定 "_shared/operation_policy.yaml"）。
func LoadOverrides(fsys fs.FS, path string) (*Policy, error) {
	base := DefaultPolicy()
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return base, nil
		}
		return nil, fmt.Errorf("policy: 读取 %s: %w", path, err)
	}
	var doc struct {
		Policy struct {
			AutoMaxRisk *int   `yaml:"auto_max_risk"`
			FullMaxRisk *int   `yaml:"full_max_risk"`
			Rules       []Rule `yaml:"rules"`
		} `yaml:"operation_policy"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("policy: %s 解析: %w", path, err)
	}
	p := &doc.Policy
	apply := func(field string, v *int, max int) (int, error) {
		if v == nil {
			return max, nil // 缺省继承基座
		}
		if *v < 0 {
			return 0, fmt.Errorf("policy: %s %s=%d 非法（负数无语义；缺省请删键，禁用写 0）", path, field, *v)
		}
		if *v > max {
			return 0, fmt.Errorf("policy: %s %s=%d 超基座（外置文件只能调低不能调高，调高=扩权须改基座走评审）", path, field, *v)
		}
		return *v, nil
	}
	auto, err := apply("auto_max_risk", p.AutoMaxRisk, base.AutoMaxRisk)
	if err != nil {
		return nil, err
	}
	full, err := apply("full_max_risk", p.FullMaxRisk, base.FullMaxRisk)
	if err != nil {
		return nil, err
	}
	// 规则逐条校验（第十轮挂账：运行期 fail-safe 兜底之外，加载期对漏写/
	// 拼错的 verdict fail-fast——启动即败，宁拒不启动不带错跑的既定纪律；
	// op_types 同纪律 oagentkit/toolgate#133：封闭词表拼错即规则永失配，
	// deny 变静默死字（fail-open），与 verdict 拼错同款失效形态）
	for i, r := range p.Rules {
		if r.Verdict != VerdictUnknown && !isValidVerdict(r.Verdict) {
			return nil, fmt.Errorf("policy: %s 规则[%d]（%s）verdict %q 非法（auto_proceed|need_human|deny|plan_only|unknown）",
				path, i, r.ID, r.Verdict)
		}
		for _, ot := range r.OpTypes {
			if !isValidOpType(ot) {
				return nil, fmt.Errorf("policy: %s 规则[%d]（%s）op_types %q 非法（tool_call|execute_step|plan_submit）——拼错即规则永失配（deny 变静默死字），启动即败（#133）",
					path, i, r.ID, ot)
			}
		}
		if r.ID == "" {
			return nil, fmt.Errorf("policy: %s 规则[%d] 缺 id", path, i)
		}
	}
	return &Policy{
		AutoMaxRisk: auto,
		FullMaxRisk: full,
		Rules:       p.Rules,
	}, nil
}
