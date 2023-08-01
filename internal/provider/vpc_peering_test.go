package provider

import (
	"reflect"
	"testing"
)

func TestCIDRList(t *testing.T) {
	type args struct {
		cidrBlocks any
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "multiple valid cidr blocks",
			args: args{
				cidrBlocks: []any{
					"10.90.5.0/25",
					"10.76.8.0/25",
				},
			},
			want: []string{
				"10.90.5.0/25",
				"10.76.8.0/25",
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			args: args{
				cidrBlocks: []any{
					"10.90.5.0/25",
					0xdeadbeef,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "not a slice",
			args: args{
				cidrBlocks: "",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CIDRList(tt.args.cidrBlocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("CIDRList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CIDRList() = %v, want %v", got, tt.want)
			}
		})
	}
}
