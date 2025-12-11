package main

import (
	"testing"
	"time"
)

func TestDailyParser(t *testing.T){
	tests := []struct{
		name string
		tag string
		expected time.Time
		wantErr bool
	}{
		{
			name: "valid begining of the year tag",
			tag: "d_2024_01_01",
			expected: time.Date(2024,1,1,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "valid may 21 2025",
			tag: "d_2025_05_21",
			expected: time.Date(2025,5,21,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "invalid daily",
			tag: "d_2025_05",
			wantErr: true,
		},
		{
			name: "invalid daily, impossible day format",
			tag: "d_2025_05_32",
			wantErr: true,
		},
		{
			name: "invalid daily, bad date format",
			tag: "d_2025_05_3232",
			wantErr: true,
		},
		{
			name: "invalid daily, impossible month",
			tag: "d_2025_14_30",
			wantErr: true,
		},
		{
			name: "invalid daily, bad year format",
			tag: "d_225_05_09",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T){
			got, err := parse_d_tag(tt.tag)
			if (err != nil) != tt.wantErr{
				t.Errorf("parse_d_tag(%s) got %v, want %v. Error: %s",tt.tag, got, tt.expected, err)
			}
			if !tt.wantErr && !got.Equal(tt.expected){
				t.Errorf("parse_d_tag(%s) = %v, want %v",tt.tag, got, tt.expected)
			}
		})
	}

}

func TestWeeklyParser(t *testing.T) {
	tests := []struct{
		name string
		tag string
		expected time.Time
		wantErr bool
	}{
		{
			name: "valid begining of the year tag",
			tag: "W_2024_01",
			expected: time.Date(2024,1,1,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "valid may 21 2025",
			tag: "w_2025_21",
			expected: time.Date(2025,5,21,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "valid Jan 29 2025",
			tag: "w_2025_05",
			expected: time.Date(2025,1,29,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "valid tag",
			tag: "W_2024_12",
			expected: time.Date(2024,3,18,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "valid ending of the year tag",
			tag: "W_2024_52",
			expected: time.Date(2024,12,23,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "valid 12/24/2025 tag",
			tag: "W_2025_52",
			expected: time.Date(2025,12,24,0,0,0,0,time.UTC),
			wantErr: false,
		},
		{
			name: "invalid tag",
			tag: "W_2025_df",
			wantErr: true,
		},
		{
			name: "invalid tag wrong year format",
			tag: "W_205_01",
			wantErr: true,
		},
		{
			name: "invalid tag wrong week format",
			tag: "W_2025_1",
			wantErr: true,
		},
		{
			name: "invalid tag impossible week",
			tag: "W_2025_54",
			wantErr: true,
		},
		{
			name: "valid tag 53 week",
			tag: "W_2023_53",
			expected: time.Date(2023,12,31,0,0,0,0,time.UTC),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T){
			got, err := parse_w_tag(tt.tag)
			if (err != nil) != tt.wantErr{
				t.Errorf("parse_w_tag(%s) error = %v, want %v. ",tt.tag, got, tt.expected)
			}
			if !tt.wantErr && !got.Equal(tt.expected){
				t.Errorf("parse_w_tag(%s) = %v, want %v",tt.tag, got, tt.expected)
			}
		})
	}

}
