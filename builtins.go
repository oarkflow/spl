package spl

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// registerBuiltinHelpers registers built-in helper functions on the engine.
func registerBuiltinHelpers(e *Engine) {
	e.RegisterHelper("slice", func(args ...any) any {
		if len(args) < 2 {
			return args
		}
		arr, ok := args[0].([]any)
		if !ok {
			return args[0]
		}
		start, _ := toInt(args[1])
		if start < 0 {
			start = 0
		}
		if start >= len(arr) {
			return []any{}
		}
		if len(args) >= 3 {
			end, _ := toInt(args[2])
			if end > len(arr) {
				end = len(arr)
			}
			if end <= start {
				return []any{}
			}
			return arr[start:end]
		}
		return arr[start:]
	})

	e.RegisterHelper("has", func(args ...any) any {
		if len(args) < 2 {
			return false
		}
		switch col := args[0].(type) {
		case []any:
			for _, item := range col {
				if fmt.Sprint(item) == fmt.Sprint(args[1]) {
					return true
				}
			}
		case map[string]any:
			key := fmt.Sprint(args[1])
			_, ok := col[key]
			return ok
		case string:
			return strings.Contains(col, fmt.Sprint(args[1]))
		}
		return false
	})

	e.RegisterHelper("keys", func(args ...any) any {
		if len(args) < 1 {
			return []any{}
		}
		switch m := args[0].(type) {
		case map[string]any:
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			result := make([]any, len(keys))
			for i, k := range keys {
				result[i] = k
			}
			return result
		}
		return []any{}
	})

	e.RegisterHelper("values", func(args ...any) any {
		if len(args) < 1 {
			return []any{}
		}
		switch m := args[0].(type) {
		case map[string]any:
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			result := make([]any, len(keys))
			for i, k := range keys {
				result[i] = m[k]
			}
			return result
		}
		return []any{}
	})

	e.RegisterHelper("defaults", func(args ...any) any {
		for _, arg := range args {
			if arg != nil {
				switch v := arg.(type) {
				case string:
					if v != "" {
						return v
					}
				case int, int64, float64:
					return arg
				case bool:
					if v {
						return v
					}
				case []any:
					if len(v) > 0 {
						return v
					}
				case map[string]any:
					if len(v) > 0 {
						return v
					}
				default:
					return arg
				}
			}
		}
		if len(args) > 0 {
			return args[len(args)-1]
		}
		return nil
	})

	e.RegisterHelper("orElse", func(args ...any) any {
		for _, arg := range args {
			if arg != nil {
				switch v := arg.(type) {
				case string:
					if v != "" {
						return v
					}
				default:
					return arg
				}
			}
		}
		return nil
	})

	e.RegisterHelper("json", func(args ...any) any {
		if len(args) < 1 {
			return "{}"
		}
		b, err := json.Marshal(args[0])
		if err != nil {
			return "{}"
		}
		return string(b)
	})

	e.RegisterHelper("contains", func(args ...any) any {
		if len(args) < 2 {
			return false
		}
		s, ok := args[0].(string)
		if !ok {
			return false
		}
		return strings.Contains(s, fmt.Sprint(args[1]))
	})

	e.RegisterHelper("replace", func(args ...any) any {
		if len(args) < 3 {
			if len(args) > 0 {
				return fmt.Sprint(args[0])
			}
			return ""
		}
		s := fmt.Sprint(args[0])
		old := fmt.Sprint(args[1])
		newStr := fmt.Sprint(args[2])
		return strings.ReplaceAll(s, old, newStr)
	})

	e.RegisterHelper("split", func(args ...any) any {
		if len(args) < 2 {
			return []any{}
		}
		s := fmt.Sprint(args[0])
		sep := fmt.Sprint(args[1])
		parts := strings.Split(s, sep)
		result := make([]any, len(parts))
		for i, p := range parts {
			result[i] = p
		}
		return result
	})

	e.RegisterHelper("join", func(args ...any) any {
		if len(args) < 2 {
			return ""
		}
		sep := fmt.Sprint(args[len(args)-1])
		parts := args[:len(args)-1]
		if len(parts) == 1 {
			switch arr := parts[0].(type) {
			case []any:
				strs := make([]string, len(arr))
				for i, v := range arr {
					strs[i] = fmt.Sprint(v)
				}
				return strings.Join(strs, sep)
			}
		}
		strs := make([]string, len(parts))
		for i, v := range parts {
			strs[i] = fmt.Sprint(v)
		}
		return strings.Join(strs, sep)
	})

	e.RegisterHelper("length", func(args ...any) any {
		if len(args) < 1 {
			return 0
		}
		switch v := args[0].(type) {
		case string:
			return len(v)
		case []any:
			return len(v)
		case map[string]any:
			return len(v)
		default:
			return 0
		}
	})

	e.RegisterHelper("lower", func(args ...any) any {
		if len(args) < 1 {
			return ""
		}
		return strings.ToLower(fmt.Sprint(args[0]))
	})

	e.RegisterHelper("upper", func(args ...any) any {
		if len(args) < 1 {
			return ""
		}
		return strings.ToUpper(fmt.Sprint(args[0]))
	})

	e.RegisterHelper("trim", func(args ...any) any {
		if len(args) < 1 {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(args[0]))
	})

	e.RegisterHelper("now", func(args ...any) any {
		if len(args) == 0 {
			return float64(time.Now().Unix())
		}
		layout := fmt.Sprint(args[0])
		return time.Now().Format(layout)
	})

	e.RegisterHelper("formatTime", func(args ...any) any {
		if len(args) < 2 {
			if len(args) > 0 {
				return fmt.Sprint(args[0])
			}
			return ""
		}
		ts := args[0]
		layout := fmt.Sprint(args[1])
		var t time.Time
		switch v := ts.(type) {
		case float64:
			t = time.Unix(int64(v), 0)
		case int64:
			t = time.Unix(v, 0)
		case int:
			t = time.Unix(int64(v), 0)
		case string:
			var unix int64
			fmt.Sscanf(v, "%d", &unix)
			t = time.Unix(unix, 0)
		default:
			return fmt.Sprint(ts)
		}
		return t.Format(layout)
	})

	e.RegisterHelper("concat", func(args ...any) any {
		result := make([]any, 0)
		for _, arg := range args {
			switch v := arg.(type) {
			case []any:
				result = append(result, v...)
			default:
				result = append(result, v)
			}
		}
		return result
	})

	e.RegisterHelper("uniq", func(args ...any) any {
		if len(args) < 1 {
			return []any{}
		}
		arr, ok := args[0].([]any)
		if !ok {
			return []any{}
		}
		seen := make(map[string]bool)
		result := make([]any, 0, len(arr))
		for _, item := range arr {
			key := fmt.Sprint(item)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}
		return result
	})

	e.RegisterHelper("sort", func(args ...any) any {
		if len(args) < 1 {
			return []any{}
		}
		arr, ok := args[0].([]any)
		if !ok {
			return []any{}
		}
		clone := make([]any, len(arr))
		copy(clone, arr)
		strs := make([]string, len(clone))
		for i, v := range clone {
			strs[i] = fmt.Sprint(v)
		}
		sort.Strings(strs)
		result := make([]any, len(strs))
		for i, s := range strs {
			result[i] = s
		}
		return result
	})
}

// toInt converts a value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	default:
		return 0, false
	}
}
